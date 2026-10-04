// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

const (
	compactionWaitMax   = 120 * time.Second
	compactionPoll      = 500 * time.Millisecond
	errorNotifyCooldown = 5 * time.Minute
	messageType         = "pig-schedule"
)

// session is the host surface the runner uses; sdk.Context implements it.
type session interface {
	Cwd() string
	IsIdle() (bool, error)
	Notify(message, level string)
	SendUserMessage(content any, deliverAs string) error
	SendCustomMessage(msg sdk.CustomMessage, opts sdk.SendMessageOptions) error
}

func isCompactionBusy(err error) bool {
	return err != nil && strings.Contains(err.Error(), "compaction is in progress")
}

type runner struct {
	store  store
	ledger ledger
	locks  *jobLocks
	trust  trustStore
	guard  *guard
	now    func() time.Time
	tick   time.Duration
	shell  func() string
	exec   executor
	wait   time.Duration
	poll   time.Duration

	waves sync.Mutex // serializes waves; auto waves drop when busy

	mu         sync.Mutex
	compacting bool
	life       context.Context
	stop       context.CancelFunc
	lastErrKey string
	lastErrAt  time.Time
}

func newRunner(s store, l ledger, locks *jobLocks, t trustStore, g *guard, now func() time.Time, shell func() string) *runner {
	life, stop := context.WithCancel(context.Background())
	return &runner{store: s, ledger: l, locks: locks, trust: t, guard: g, now: now, tick: defaultTick, shell: shell,
		exec: runProcessGroup, wait: compactionWaitMax, poll: compactionPoll, life: life, stop: stop}
}

func (r *runner) setCompacting(on bool) {
	r.mu.Lock()
	r.compacting = on
	r.mu.Unlock()
}

func (r *runner) isCompacting() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.compacting
}

// close cancels waits; deliveries after close are refused.
func (r *runner) close() {
	r.stop()
	r.guard.clear()
}

func (r *runner) closed() bool { return r.life.Err() != nil }

// fireDue runs one wave. run_now waits for an active wave; automatic waves
// are dropped while another wave runs.
func (r *runner) fireDue(s session, source string, ids []string) ([]Job, error) {
	if source == sourceRunNow {
		r.waves.Lock()
	} else if !r.waves.TryLock() {
		return nil, nil
	}
	defer r.waves.Unlock()
	if r.closed() {
		return nil, errors.New("scheduler is shutting down")
	}
	jobs, err := r.runWave(s, source, ids)
	if err != nil && source != sourceRunNow {
		var se storeError
		if errors.As(err, &se) {
			r.emitError(s, "store error: "+err.Error(), "store:"+err.Error())
		} else {
			r.emitError(s, "runner error: "+err.Error(), "runner:"+err.Error())
		}
		return nil, nil
	}
	return jobs, err
}

func (r *runner) eligible(job Job, cwd string) bool {
	if job.Scope != scopeProject {
		return true
	}
	root := job.ProjectPath
	if root == "" {
		root = cwd
	}
	return r.trust.isTrusted(root)
}

func (r *runner) runWave(s session, source string, ids []string) ([]Job, error) {
	now := r.now()
	cwd := s.Cwd()
	var candidates []Job
	if len(ids) > 0 {
		for _, id := range ids {
			job, err := r.store.get(id, cwd)
			if err != nil {
				return nil, err
			}
			if job != nil {
				candidates = append(candidates, *job)
			}
		}
	} else {
		if source == sourceTick {
			if idle, err := s.IsIdle(); err != nil || !idle {
				return nil, nil
			}
		}
		due, err := r.store.due(cwd, now)
		if err != nil {
			return nil, err
		}
		candidates = due
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	var gated []Job
	if source != sourceRunNow {
		allowed := candidates[:0:0]
		for _, job := range candidates {
			if r.eligible(job, cwd) {
				allowed = append(allowed, job)
			} else {
				gated = append(gated, job)
			}
		}
		candidates = allowed
		if len(candidates) == 0 {
			if source == sourceSessionStart && len(gated) > 0 {
				r.notifyTrustGate(s, gated)
			}
			return nil, nil
		}
	}
	maxFires := len(candidates)
	switch source {
	case sourceSessionStart:
		maxFires = maxFiresPerSessionStart
	case sourceTick:
		maxFires = maxFiresPerTick
	}
	var updated []Job
	attempts := 0
	for _, job := range candidates {
		if r.closed() {
			break
		}
		deliverAs := ""
		if attempts > 0 {
			deliverAs = "followUp"
		}
		forced := source == sourceRunNow
		result, err := r.processOne(s, job, source, forced, deliverAs, forced || attempts < maxFires)
		if err != nil {
			return updated, err
		}
		if result != nil {
			updated = append(updated, *result)
			if status := deref(result.LastStatus); status == statusOK || status == statusError {
				attempts++
			}
		}
	}
	if source == sourceSessionStart && len(gated) > 0 {
		r.notifyTrustGate(s, gated)
	}
	return updated, nil
}

func (r *runner) emitError(s session, detail, key string) {
	r.mu.Lock()
	now := time.Now()
	if key == r.lastErrKey && now.Sub(r.lastErrAt) < errorNotifyCooldown {
		r.mu.Unlock()
		return
	}
	r.lastErrKey, r.lastErrAt = key, now
	r.mu.Unlock()
	s.Notify(labelPrefix+" "+detail, "error")
}

func (r *runner) notifyTrustGate(s session, gated []Job) {
	names := make([]string, len(gated))
	for i, job := range gated {
		names[i] = fmt.Sprintf("%q (%s)", job.Name, job.ID)
	}
	s.Notify(fmt.Sprintf("%s held back %d project job(s) — this project is not trusted: %s. Inspect .pig/schedule.json (untrusted files can carry shell jobs), then allow auto-fire with: schedule action=trust",
		labelPrefix, len(gated), strings.Join(names, ", ")), "info")
}

func (r *runner) alreadyDelivered(job Job, key string) bool {
	if job.LastIdempotencyKey == key && deref(job.LastStatus) == statusOK {
		return true
	}
	return r.ledger.wasDelivered(key)
}

func (r *runner) record(job Job, runID, key, source, status, detail, started string) {
	r.ledger.append(Run{RunID: runID, JobID: job.ID, JobName: job.Name, Scope: job.Scope, ProjectPath: job.ProjectPath,
		IdempotencyKey: key, Source: source, Status: status, StartedAt: started, EndedAt: isoTime(r.now()), Detail: detail,
		Tier: job.Tier, MissedWindow: job.MissedWindow, Action: job.Action})
}

// sendAgentMessage submits a user message after any in-flight compaction,
// bounded by r.wait. PiG reports an extension submission rejected during
// compaction to the user, not to the extension, so the compaction events are
// the primary signal and a returned busy error is retried as a backstop.
func (r *runner) sendAgentMessage(s session, body, deliverAs string) error {
	deadline := time.Now().Add(r.wait)
	for {
		for r.isCompacting() {
			if time.Now().After(deadline) {
				return fmt.Errorf("context compaction did not finish within %s", r.wait)
			}
			select {
			case <-r.life.Done():
				return errors.New("scheduler is shutting down")
			case <-time.After(r.poll):
			}
		}
		if r.closed() {
			return errors.New("scheduler is shutting down")
		}
		mode := deliverAs
		if mode == "" {
			if idle, err := s.IsIdle(); err != nil || !idle {
				mode = "followUp"
			}
		}
		err := s.SendUserMessage(body, mode)
		if !isCompactionBusy(err) {
			return err
		}
		if time.Now().After(deadline) {
			return err
		}
		r.setCompacting(true)
	}
}

type delivery struct {
	detail string
	shell  *ShellResult
}

func (r *runner) deliver(s session, job Job, runID, source string, forced bool, deliverAs string) (delivery, error) {
	noTurn := sdk.SendMessageOptions{TriggerTurn: sdk.Bool(false)}
	switch job.Action {
	case kindNotify:
		msg := notifyLabel(job)
		s.Notify(msg, "info")
		_ = s.SendCustomMessage(sdk.CustomMessage{CustomType: messageType, Content: msg, Display: true,
			Details: map[string]any{"jobId": job.ID, "action": kindNotify, "runId": runID}}, noTurn)
		return delivery{detail: kindNotify}, nil
	case kindMessage:
		body := jsTrim(job.Prompt)
		if body == "" {
			body = job.Name
		}
		if err := s.SendCustomMessage(sdk.CustomMessage{CustomType: messageType, Content: body, Display: true,
			Details: map[string]any{"jobId": job.ID, "action": kindMessage, "runId": runID}}, noTurn); err != nil {
			return delivery{}, err
		}
		return delivery{detail: kindMessage}, nil
	case kindShell:
		return r.deliverShell(s, job, runID, source, forced, deliverAs)
	}
	body := buildFirePrompt(job, runID, source, forced)
	release := r.guard.reserve(body, job.Tier)
	if err := r.sendAgentMessage(s, body, deliverAs); err != nil {
		release()
		return delivery{}, err
	}
	return delivery{detail: kindPrompt}, nil
}

func (r *runner) deliverShell(s session, job Job, runID, source string, forced bool, deliverAs string) (delivery, error) {
	command := strings.TrimSpace(job.Command)
	if command == "" {
		return delivery{}, fmt.Errorf("shell job %q has no command", job.Name)
	}
	// Global jobs run in the session cwd; prefer absolute commands for them.
	cwd := job.ProjectPath
	if cwd == "" {
		cwd = s.Cwd()
	}
	timeout := job.TimeoutMs
	if timeout <= 0 {
		timeout = defaultShellTimeoutMs
	}
	s.Notify(fmt.Sprintf("%s running shell %q: %s", labelPrefix, oneLine(job.Name), oneLine(command)), "info")
	run, err := r.exec(r.life, r.shell(), []string{"-lc", command}, cwd, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return delivery{}, err
	}
	if r.closed() {
		return delivery{}, errors.New("scheduler shut down during the shell job")
	}
	result := ShellResult{OK: run.code == 0 && !run.killed, Command: command, Cwd: cwd, TimeoutMs: timeout, Code: run.code,
		Killed: run.killed, Stdout: truncateOutput(run.stdout), Stderr: truncateOutput(run.stderr)}
	persisted := result
	persisted.Stdout, persisted.Stderr = redactSecrets(result.Stdout), redactSecrets(result.Stderr)
	killed := ""
	if result.Killed {
		killed = " (killed)"
	}
	_ = s.SendCustomMessage(sdk.CustomMessage{CustomType: messageType, Display: true,
		Content: fmt.Sprintf("Shell %q exit %d%s: %s", job.Name, result.Code, killed, command),
		Details: map[string]any{"jobId": job.ID, "action": kindShell, "runId": runID, "result": persisted}},
		sdk.SendMessageOptions{TriggerTurn: sdk.Bool(false)})
	woke := false
	if shouldWake(job, result) {
		if instruction := followUpFor(job, result); instruction != "" {
			body := buildShellFollowUp(job, runID, source, forced, result, instruction)
			release := r.guard.reserve(body, job.Tier)
			if err := r.sendAgentMessage(s, body, deliverAs); err != nil {
				release()
				return delivery{shell: &persisted}, err
			}
			woke = true
		}
	}
	detail := fmt.Sprintf("shell exit=%d", result.Code)
	if result.Killed {
		detail += " killed"
	}
	if woke {
		detail += " woke"
	}
	return delivery{detail: detail, shell: &persisted}, nil
}

// processOne fires or skips one job. A nil result means the job stays due
// (over the fire cap, locked) or was cancelled or disabled concurrently.
func (r *runner) processOne(s session, job Job, source string, forced bool, deliverAs string, allowFire bool) (*Job, error) {
	at := r.now()
	started := isoTime(at)
	runID := newRunID()
	cwd := s.Cwd()
	key := idempotencyKey(job)
	if forced {
		key = job.ID + ":force:" + runID
	}
	skip := func(subject Job, subjectKey, reason string) (*Job, error) {
		updated, err := r.store.markAttempt(subject, cwd, at, statusSkipped, reason, subjectKey, true, nil)
		if err != nil {
			return nil, err
		}
		r.record(subject, runID, subjectKey, source, "skipped", reason, started)
		return updated, nil
	}
	if !forced {
		if r.alreadyDelivered(job, key) {
			return skip(job, key, "idempotent_replay")
		}
		if d := decideDue(job, at, r.tick); !d.fire {
			return skip(job, key, d.reason)
		}
	}
	if !allowFire {
		return nil, nil
	}
	release := r.locks.tryAcquire(job.ID)
	if release == nil {
		return nil, nil
	}
	defer release()

	// Re-check the freshest row after locking: it may have been cancelled,
	// disabled or already delivered by another session.
	fresh, err := r.store.get(job.ID, cwd)
	if err != nil {
		return nil, err
	}
	if fresh == nil || (!forced && !fresh.Enabled) {
		return nil, nil
	}
	freshKey := key
	if !forced {
		freshKey = idempotencyKey(*fresh)
		if r.alreadyDelivered(*fresh, freshKey) {
			return skip(*fresh, freshKey, "idempotent_replay_post_lock")
		}
	}

	result, deliverErr := r.deliver(s, *fresh, runID, source, forced, deliverAs)
	status, ledgerStatus, detail := statusOK, "delivered", result.detail
	if deliverErr != nil {
		status, ledgerStatus, detail = statusError, "error", deliverErr.Error()
		s.Notify(fmt.Sprintf("%s failed to fire %q: %s", labelPrefix, oneLine(fresh.Name), deliverErr.Error()), "error")
	}
	// Advance the durable store first; the ledger is best-effort.
	updated, err := r.store.markAttempt(*fresh, cwd, at, status, errorText(deliverErr), freshKey, true, result.shell)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		r.record(*fresh, runID, freshKey, source, ledgerStatus, detail+" (job cancelled during run)", started)
		return nil, nil
	}
	if term := terminalReason(*updated, updated.RunCount); term != "" {
		terminated, err := r.store.terminate(*updated, cwd, term, at)
		if err != nil {
			return nil, err
		}
		if terminated != nil {
			updated = terminated
		}
		detail += " terminated:" + term
	}
	r.record(*fresh, runID, freshKey, source, ledgerStatus, detail, started)
	return updated, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
