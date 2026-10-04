// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

type sentMessage struct{ body, deliverAs string }

type fakeSession struct {
	mu       sync.Mutex
	cwd      string
	idle     bool
	notes    []string
	sent     []sentMessage
	custom   []sdk.CustomMessage
	sendErr  func(attempt int) error
	onSend   func()
	attempts int
	turn     bool
}

func (f *fakeSession) Cwd() string { return f.cwd }
func (f *fakeSession) IsIdle() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.idle, nil
}
func (f *fakeSession) Notify(message, level string) {
	f.mu.Lock()
	f.notes = append(f.notes, level+": "+message)
	f.mu.Unlock()
}
func (f *fakeSession) SendUserMessage(content any, deliverAs string) error {
	f.mu.Lock()
	f.attempts++
	attempt := f.attempts
	errFn, hook := f.sendErr, f.onSend
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	if errFn != nil {
		if err := errFn(attempt); err != nil {
			return err
		}
	}
	f.mu.Lock()
	f.sent = append(f.sent, sentMessage{content.(string), deliverAs})
	f.mu.Unlock()
	return nil
}
func (f *fakeSession) SendCustomMessage(msg sdk.CustomMessage, opts sdk.SendMessageOptions) error {
	if opts.TriggerTurn == nil || *opts.TriggerTurn {
		panic("custom messages must never trigger a turn")
	}
	f.mu.Lock()
	f.custom = append(f.custom, msg)
	f.mu.Unlock()
	return nil
}
func (f *fakeSession) scheduledTurn() bool { return f.turn }

type harness struct {
	t      *testing.T
	runner *runner
	store  store
	cwd    string
	now    time.Time
	shell  string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	h := &harness{t: t, cwd: filepath.Join(root, "project"), now: fixedNow, shell: "bash"}
	_ = os.MkdirAll(h.cwd, 0o700)
	p := newPaths(filepath.Join(root, "home"))
	clock := func() time.Time { return h.now }
	h.store = store{paths: p, now: clock}
	h.runner = newRunner(h.store, ledger{path: p.runsFile, maxBytes: maxLedgerBytes}, newJobLocks(p.lockDir, clock),
		trustStore{path: p.trustFile}, &guard{legacy: func() bool { return false }}, clock, func() string { return h.shell })
	h.runner.wait, h.runner.poll = 300*time.Millisecond, 5*time.Millisecond
	return h
}

func (h *harness) session() *fakeSession { return &fakeSession{cwd: h.cwd, idle: true} }

func (h *harness) add(mutate func(*Job)) Job {
	h.t.Helper()
	job := newTestJob(newJobID(), scopeGlobal, "")
	job.NextRunAt = isoTime(h.now.Add(-time.Second))
	if mutate != nil {
		mutate(&job)
	}
	if _, err := h.store.create(job, h.cwd); err != nil {
		h.t.Fatal(err)
	}
	return job
}

func (h *harness) get(id string) *Job {
	job, err := h.store.get(id, h.cwd)
	if err != nil {
		h.t.Fatal(err)
	}
	return job
}

func (h *harness) fire(s *fakeSession, source string, ids ...string) []Job {
	h.t.Helper()
	jobs, err := h.runner.fireDue(s, source, ids)
	if err != nil {
		h.t.Fatal(err)
	}
	return jobs
}

func TestFiresDueJobOnceAndRecords(t *testing.T) {
	h := newHarness(t)
	job := h.add(nil)
	s := h.session()
	fired := h.fire(s, sourceTick)
	if len(fired) != 1 || len(s.sent) != 1 || s.sent[0].deliverAs != "" || !strings.Contains(s.sent[0].body, "jobId: "+job.ID) {
		t.Fatalf("%+v %+v", fired, s.sent)
	}
	got := h.get(job.ID)
	if got.RunCount != 1 || deref(got.LastStatus) != statusOK || got.LastIdempotencyKey != job.ID+":"+job.NextRunAt || got.NextRunAt != isoTime(h.now.Add(time.Hour)) {
		t.Fatalf("%+v", got)
	}
	if tier, ok := h.runner.guard.tierFor(s.sent[0].body); !ok || tier != tierReadOnly {
		t.Fatal("prompt reserved with its tier")
	}
	runs := h.runner.ledger.history(job.ID, 10)
	if len(runs) != 1 || runs[0].Status != "delivered" || runs[0].Source != sourceTick || runs[0].Detail != kindPrompt {
		t.Fatalf("%+v", runs)
	}
	if len(h.fire(s, sourceTick)) != 0 || len(s.sent) != 1 {
		t.Fatal("not due again until the next slot")
	}
}

func TestIdempotentReplayAndRunNow(t *testing.T) {
	h := newHarness(t)
	job := h.add(nil)
	key := job.ID + ":" + job.NextRunAt
	// Another session already delivered this slot but crashed before advancing.
	h.runner.ledger.append(Run{JobID: job.ID, IdempotencyKey: key, Status: "delivered"})
	s := h.session()
	fired := h.fire(s, sourceTick)
	if len(fired) != 1 || len(s.sent) != 0 || deref(fired[0].LastStatus) != statusSkipped || fired[0].LastError != "idempotent_replay" || fired[0].RunCount != 0 {
		t.Fatalf("%+v", fired)
	}
	fired = h.fire(s, sourceRunNow, job.ID)
	fired2 := h.fire(s, sourceRunNow, job.ID)
	if len(s.sent) != 2 || fired[0].LastIdempotencyKey == fired2[0].LastIdempotencyKey || !strings.Contains(fired[0].LastIdempotencyKey, ":force:") {
		t.Fatalf("run_now always attempts with a unique key: %+v %+v", fired, fired2)
	}
	if !strings.Contains(s.sent[0].body, "source: force-run") {
		t.Fatal(s.sent[0].body)
	}
}

func TestTrustGate(t *testing.T) {
	h := newHarness(t)
	project := h.add(func(j *Job) { j.Scope, j.ProjectPath = scopeProject, h.cwd })
	global := h.add(nil)
	s := h.session()
	fired := h.fire(s, sourceSessionStart)
	if len(fired) != 1 || fired[0].ID != global.ID {
		t.Fatalf("only the global job fires: %+v", fired)
	}
	if got := h.get(project.ID); got.RunCount != 0 || got.NextRunAt != project.NextRunAt || len(h.runner.ledger.history(project.ID, 5)) != 0 {
		t.Fatal("gated jobs stay untouched and unlogged")
	}
	if notes := strings.Join(s.notes, "\n"); !strings.Contains(notes, "held back 1 project job(s)") || !strings.Contains(notes, project.ID) {
		t.Fatal(notes)
	}
	tick := h.session()
	h.fire(tick, sourceTick)
	if len(tick.notes) != 0 || len(tick.sent) != 0 {
		t.Fatal("ticks neither fire gated jobs nor repeat the notice", tick.notes)
	}
	if fired := h.fire(tick, sourceRunNow, project.ID); len(fired) != 1 || len(tick.sent) != 1 {
		t.Fatal("run_now bypasses the gate")
	}
	other := h.add(func(j *Job) { j.Scope, j.ProjectPath = scopeProject, h.cwd })
	_ = h.runner.trust.trust(h.cwd, h.now)
	if fired := h.fire(h.session(), sourceTick); len(fired) != 1 || fired[0].ID != other.ID {
		t.Fatalf("trusted project fires: %+v", fired)
	}
}

func TestRelabeledProjectRowIsStillGated(t *testing.T) {
	h := newHarness(t)
	file := projectFile(h.cwd)
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	_ = os.WriteFile(file, []byte(fmt.Sprintf(`{"version":1,"jobs":[{"id":"evil","name":"x","scope":"global","action":"shell","command":"touch pwned","tier":"mutate","schedule":{"type":"interval","everyMs":60000,"every":"1m"},"enabled":true,"nextRunAt":"%s"}]}`, isoTime(h.now.Add(-time.Minute)))), 0o600)
	s := h.session()
	ran := false
	h.runner.exec = func(context.Context, string, []string, string, time.Duration) (execResult, error) {
		ran = true
		return execResult{}, nil
	}
	if fired := h.fire(s, sourceSessionStart); len(fired) != 0 || ran {
		t.Fatal("a cloned project row labeled global must not auto-run")
	}
}

func TestCapsAndFollowUps(t *testing.T) {
	h := newHarness(t)
	for range 7 {
		h.add(nil)
	}
	s := h.session()
	if fired := h.fire(s, sourceSessionStart); len(fired) != maxFiresPerSessionStart || len(s.sent) != 5 {
		t.Fatalf("%d fired", len(fired))
	}
	if s.sent[0].deliverAs != "" || s.sent[1].deliverAs != "followUp" || s.sent[4].deliverAs != "followUp" {
		t.Fatalf("%+v", s.sent)
	}
	if due, _ := h.store.due(h.cwd, h.now); len(due) != 2 {
		t.Fatal("over-cap jobs stay due without ledger spam", len(due))
	}
	if len(h.runner.ledger.recent()) != 5 {
		t.Fatal("no ledger rows for over-cap jobs")
	}
	busy := h.session()
	busy.idle = false
	if len(h.fire(busy, sourceTick)) != 0 || len(busy.sent) != 0 {
		t.Fatal("tick is a no-op while busy")
	}
	tick := h.session()
	if len(h.fire(tick, sourceTick)) != 2 {
		t.Fatal("next idle tick drains the remaining jobs")
	}
}

func TestBusyRunNowDeliversAsFollowUp(t *testing.T) {
	h := newHarness(t)
	job := h.add(nil)
	s := h.session()
	s.idle = false
	h.fire(s, sourceRunNow, job.ID)
	if len(s.sent) != 1 || s.sent[0].deliverAs != "followUp" {
		t.Fatal(s.sent)
	}
}

func TestSkipPolicyAndDeliveryErrors(t *testing.T) {
	h := newHarness(t)
	skip := h.add(func(j *Job) { j.MissedWindow, j.NextRunAt = missedSkip, isoTime(h.now.Add(-3*time.Hour)) })
	s := h.session()
	fired := h.fire(s, sourceTick)
	if len(fired) != 1 || len(s.sent) != 0 || deref(fired[0].LastStatus) != statusSkipped || fired[0].RunCount != 0 || fired[0].NextRunAt != isoTime(h.now.Add(time.Hour)) {
		t.Fatalf("%+v", fired)
	}
	_, _ = h.store.remove(skip.ID, h.cwd)
	broken := h.add(nil)
	fail := h.session()
	fail.sendErr = func(int) error { return errors.New("boom") }
	fired = h.fire(fail, sourceTick)
	if len(fired) != 1 || deref(fired[0].LastStatus) != statusError || fired[0].LastError != "boom" || fired[0].RunCount != 1 || fired[0].NextRunAt == broken.NextRunAt {
		t.Fatalf("errors advance and count: %+v", fired)
	}
	if !strings.Contains(strings.Join(fail.notes, "\n"), `failed to fire "n": boom`) || !h.runner.guard.empty() {
		t.Fatal("error notified; failed submission releases its reservation", fail.notes)
	}
	if runs := h.runner.ledger.history(broken.ID, 1); runs[0].Status != "error" || runs[0].Detail != "boom" {
		t.Fatal(runs)
	}
}

func TestLockContentionAndWaveSerialization(t *testing.T) {
	h := newHarness(t)
	job := h.add(nil)
	other := newJobLocks(h.store.paths.lockDir, func() time.Time { return h.now })
	release := other.tryAcquire(job.ID)
	s := h.session()
	if fired := h.fire(s, sourceTick); len(fired) != 0 || len(s.sent) != 0 || h.get(job.ID).NextRunAt != job.NextRunAt {
		t.Fatal("locked: no fire, no advance")
	}
	release()
	if len(h.fire(s, sourceTick)) != 1 {
		t.Fatal("fires after release")
	}
	h.runner.waves.Lock()
	if jobs, _ := h.runner.fireDue(s, sourceTick, nil); jobs != nil {
		t.Fatal("auto wave dropped while another is active")
	}
	done := make(chan []Job)
	go func() {
		jobs, _ := h.runner.fireDue(s, sourceRunNow, []string{job.ID})
		done <- jobs
	}()
	select {
	case <-done:
		t.Fatal("run_now must wait for the active wave")
	case <-time.After(50 * time.Millisecond):
	}
	h.runner.waves.Unlock()
	if jobs := <-done; len(jobs) != 1 {
		t.Fatal("run_now delivered after waiting")
	}
}

func TestConcurrentDisableAndCancelDuringDelivery(t *testing.T) {
	h := newHarness(t)
	job := h.add(nil)
	s := h.session()
	s.onSend = func() {
		fresh := h.get(job.ID)
		if _, err := h.store.setEnabled(*fresh, h.cwd, false); err != nil {
			t.Error(err)
		}
	}
	fired := h.fire(s, sourceTick)
	if len(fired) != 1 || fired[0].Enabled || fired[0].RunCount != 1 {
		t.Fatalf("stale completion keeps the concurrent disable: %+v", fired)
	}
	cancelled := h.add(nil)
	c := h.session()
	c.onSend = func() { _, _ = h.store.remove(cancelled.ID, h.cwd) }
	if fired := h.fire(c, sourceTick); len(fired) != 0 || h.get(cancelled.ID) != nil {
		t.Fatal("a job cancelled during delivery is not resurrected")
	}
	if runs := h.runner.ledger.history(cancelled.ID, 1); len(runs) != 1 || !strings.Contains(runs[0].Detail, "cancelled during run") {
		t.Fatal(runs)
	}
	disabled := h.add(nil)
	// Disabled after the due scan but before the lock: never delivered.
	d := h.session()
	h.runner.waves.Lock()
	candidate := *h.get(disabled.ID)
	h.runner.waves.Unlock()
	_, _ = h.store.setEnabled(candidate, h.cwd, false)
	if got, err := h.runner.processOne(d, candidate, sourceTick, false, "", true); err != nil || got != nil || len(d.sent) != 0 {
		t.Fatal("disabled between scan and lock must not fire", got, err)
	}
}

func TestOnceAndMaxRunsTerminate(t *testing.T) {
	h := newHarness(t)
	once := h.add(func(j *Job) { j.Schedule = Spec{Type: "once", DelayMs: 60_000, Delay: "1m"} })
	s := h.session()
	fired := h.fire(s, sourceTick)
	if len(fired) != 1 || fired[0].Enabled || deref(fired[0].Terminated) != "once" {
		t.Fatalf("%+v", fired)
	}
	h.now = h.now.Add(2 * time.Hour)
	if len(h.fire(s, sourceTick)) != 0 {
		t.Fatal("terminated job never fires again")
	}
	if runs := h.runner.ledger.history(once.ID, 1); !strings.HasSuffix(runs[0].Detail, "terminated:once") {
		t.Fatal(runs)
	}
	capped := h.add(func(j *Job) { j.MaxRuns = 2; j.NextRunAt = isoTime(h.now.Add(-time.Second)) })
	for i := range 3 {
		h.now = h.now.Add(2 * time.Hour)
		h.fire(s, sourceTick)
		got := h.get(capped.ID)
		if i == 0 && !got.Enabled || i >= 1 && (got.Enabled || deref(got.Terminated) != "maxRuns" || got.RunCount != 2) {
			t.Fatalf("round %d: %+v", i, got)
		}
	}
	erroring := h.add(func(j *Job) {
		j.Schedule = Spec{Type: "once", DelayMs: 60_000, Delay: "1m"}
		j.NextRunAt = isoTime(h.now.Add(-time.Second))
	})
	fail := h.session()
	fail.sendErr = func(int) error { return errors.New("x") }
	h.fire(fail, sourceTick)
	if got := h.get(erroring.ID); deref(got.Terminated) != "once" {
		t.Fatal("an erroring once job still terminates")
	}
}

func TestNotifyMessageAndShellKinds(t *testing.T) {
	h := newHarness(t)
	h.add(func(j *Job) { j.Action, j.Prompt, j.Name = kindNotify, "stretch\x1b[2J", "break" })
	h.add(func(j *Job) { j.Action, j.Prompt = kindMessage, "  note  " })
	s := h.session()
	h.fire(s, sourceTick)
	if len(s.sent) != 0 || !h.runner.guard.empty() {
		t.Fatal("notify/message start no agent turn")
	}
	if len(s.custom) != 2 || s.custom[0].Content != "[pig-schedule] break: stretch [2J" || s.custom[1].Content != "note" || s.custom[1].CustomType != messageType {
		t.Fatalf("%+v", s.custom)
	}
	if !strings.Contains(strings.Join(s.notes, "\n"), "info: [pig-schedule] break: stretch") {
		t.Fatal(s.notes)
	}
}

func TestShellDelivery(t *testing.T) {
	secret := "ghp_" + strings.Repeat("01", 18)
	cases := []struct {
		name     string
		job      func(*Job)
		result   execResult
		wantWake string // "" means no wake
	}{
		{"never", func(j *Job) { j.WakeOn = "never" }, execResult{code: 1}, ""},
		{"failure-on-fail", func(j *Job) { j.WakeOn, j.FailurePrompt = "failure", "fix it" }, execResult{code: 2, stdout: "token " + secret}, "fix it"},
		{"failure-on-success", func(j *Job) { j.WakeOn, j.FailurePrompt = "failure", "fix it" }, execResult{code: 0}, ""},
		{"success", func(j *Job) { j.WakeOn, j.SuccessPrompt = "success", "celebrate" }, execResult{code: 0}, "celebrate"},
		{"always-generic", func(j *Job) { j.WakeOn = "always" }, execResult{code: 0}, genericFollowUp},
		{"killed", func(j *Job) { j.WakeOn, j.FailurePrompt = "failure", "timed out" }, execResult{code: -1, killed: true}, "timed out"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.shell = "/custom/bash"
			job := h.add(func(j *Job) {
				j.Action, j.Command, j.Tier, j.Prompt, j.TimeoutMs = kindShell, "make check", tierMutate, "", 5000
				tc.job(j)
			})
			s := h.session()
			result := tc.result
			var calls []string
			h.runner.exec = func(_ context.Context, shell string, args []string, cwd string, timeout time.Duration) (execResult, error) {
				calls = append(calls, fmt.Sprint(shell, args, cwd, timeout))
				return result, nil
			}
			h.fire(s, sourceTick)
			if len(calls) != 1 || calls[0] != fmt.Sprint("/custom/bash", []string{"-lc", "make check"}, h.cwd, 5*time.Second) {
				t.Fatalf("%v", calls)
			}
			got := h.get(job.ID)
			if got.LastShell == nil || got.LastShell.Code != result.code || got.LastShell.Killed != result.killed || strings.Contains(got.LastShell.Stdout, secret) {
				t.Fatalf("persisted shell result (redacted): %+v", got.LastShell)
			}
			if len(s.custom) != 1 || strings.Contains(fmt.Sprint(s.custom[0].Details), secret) {
				t.Fatal("session record is redacted", s.custom)
			}
			if tc.wantWake == "" {
				if len(s.sent) != 0 {
					t.Fatal("unexpected wake", s.sent)
				}
				return
			}
			if len(s.sent) != 1 || !strings.Contains(s.sent[0].body, "## Instruction\n"+tc.wantWake) {
				t.Fatalf("%+v", s.sent)
			}
			if result.stdout != "" && !strings.Contains(s.sent[0].body, secret) {
				t.Fatal("the transient follow-up keeps full output")
			}
			if tier, ok := h.runner.guard.tierFor(s.sent[0].body); !ok || tier != tierMutate {
				t.Fatal("shell follow-up is a scheduled mutate turn")
			}
			if runs := h.runner.ledger.history(job.ID, 1); !strings.HasSuffix(runs[0].Detail, "woke") {
				t.Fatal(runs)
			}
		})
	}
	h := newHarness(t)
	job := h.add(func(j *Job) { j.Action, j.Command, j.Tier = kindShell, "x", tierMutate })
	s := h.session()
	h.runner.exec = func(context.Context, string, []string, string, time.Duration) (execResult, error) {
		return execResult{}, errors.New("exec failed")
	}
	h.fire(s, sourceTick)
	if got := h.get(job.ID); deref(got.LastStatus) != statusError || got.LastError != "exec failed" {
		t.Fatal(got)
	}
}

func TestCompactionWait(t *testing.T) {
	h := newHarness(t)
	h.add(nil)
	h.runner.setCompacting(true)
	s := h.session()
	go func() {
		time.Sleep(40 * time.Millisecond)
		h.runner.setCompacting(false)
	}()
	start := time.Now()
	h.fire(s, sourceTick)
	if len(s.sent) != 1 || time.Since(start) < 30*time.Millisecond {
		t.Fatal("waits for compaction to end, then delivers")
	}

	h = newHarness(t)
	job := h.add(nil)
	h.runner.setCompacting(true)
	s = h.session()
	h.fire(s, sourceTick)
	if got := h.get(job.ID); deref(got.LastStatus) != statusError || !strings.Contains(got.LastError, "compaction did not finish") || len(s.sent) != 0 || got.NextRunAt == job.NextRunAt {
		t.Fatalf("bounded wait degrades to the error path: %+v", got)
	}

	h = newHarness(t)
	h.add(nil)
	s = h.session()
	s.sendErr = func(attempt int) error {
		if attempt == 1 {
			go func() { time.Sleep(20 * time.Millisecond); h.runner.setCompacting(false) }()
			return errors.New("Cannot submit a prompt while compaction is in progress. Wait for compaction to finish and retry.")
		}
		return nil
	}
	h.fire(s, sourceTick)
	if len(s.sent) != 1 || s.attempts != 2 {
		t.Fatal("a returned busy error is retried", s.attempts)
	}

	h = newHarness(t)
	h.add(nil)
	s = h.session()
	s.sendErr = func(int) error { return errors.New("other failure") }
	h.fire(s, sourceTick)
	if s.attempts != 1 {
		t.Fatal("other send failures fail fast")
	}

	h = newHarness(t)
	h.add(nil)
	h.runner.setCompacting(true)
	s = h.session()
	go func() { time.Sleep(20 * time.Millisecond); h.runner.close() }()
	start = time.Now()
	h.fire(s, sourceTick)
	if len(s.sent) != 0 || time.Since(start) > 200*time.Millisecond {
		t.Fatal("shutdown cancels a compaction wait promptly")
	}
	if jobs, err := h.runner.fireDue(s, sourceRunNow, nil); jobs != nil || err == nil {
		t.Fatal("a closed runner refuses new waves")
	}
}

func TestStoreErrorsAreNotifiedOnce(t *testing.T) {
	h := newHarness(t)
	_ = os.MkdirAll(filepath.Dir(h.store.paths.globalFile), 0o700)
	s := h.session()
	for range 2 {
		_ = os.WriteFile(h.store.paths.globalFile, []byte("{bad"), 0o600)
		if jobs, err := h.runner.fireDue(s, sourceTick, nil); jobs != nil || err != nil {
			t.Fatal("automatic waves swallow store errors", err)
		}
	}
	if len(s.notes) != 1 || !strings.Contains(s.notes[0], "store error: Schedule store unreadable (invalid JSON)") {
		t.Fatal(s.notes)
	}
}

// Reproduces queued-tiers-activate-only-for-their-own-turn.
func TestQueuedTiersActivateOnlyForTheirOwnTurn(t *testing.T) {
	g := &guard{legacy: func() bool { return false }}
	readOnly := "[scheduled-task]\nrunId: a\n...read_only"
	mutate := "[scheduled-task]\nrunId: b\n...mutate"
	g.reserve(readOnly, tierReadOnly)
	g.reserve(mutate, tierMutate)
	user := func(text string) map[string]any {
		return map[string]any{"type": "message", "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": text}}}}
	}
	assistant := map[string]any{"type": "message", "message": map[string]any{"role": "assistant", "content": "ok"}}
	tierAt := func(branch []map[string]any) string {
		text, ok := latestUserText(branch)
		if !ok {
			return ""
		}
		tier, _ := g.tierFor(text)
		return tier
	}
	// First scheduled turn (read_only) is active while the mutate follow-up is queued.
	branch := []map[string]any{user("hello"), assistant, user(readOnly)}
	if tierAt(branch) != tierReadOnly || !g.check(tierAt(branch), "bash", nil).block {
		t.Fatal("the read_only turn must not inherit the queued mutate tier")
	}
	branch = append(branch, assistant, user(mutate))
	if tierAt(branch) != tierMutate || g.check(tierAt(branch), "bash", nil).block {
		t.Fatal("the mutate follow-up owns its own turn")
	}
	// An ordinary user turn never inherits a scheduled restriction.
	if tierAt(append(branch, assistant, user("my own request"))) != "" {
		t.Fatal("user turn inherited a scheduled tier")
	}
	// An unrelated settlement keeps reservations whose prompt has not started.
	g2 := &guard{legacy: func() bool { return false }}
	g2.reserve(readOnly, tierReadOnly)
	g2.settled([]map[string]any{user("unrelated turn")})
	if g2.empty() {
		t.Fatal("unrelated settlement discarded a queued reservation")
	}
	g2.settled([]map[string]any{user(readOnly)})
	if !g2.empty() {
		t.Fatal("settlement after the scheduled turn releases it")
	}
	g.settled(branch)
	if !g.empty() {
		t.Fatal("both started reservations released at settlement")
	}
	for range maxReservations + 3 {
		g.reserve(newRunID(), tierReadOnly)
	}
	if len(g.reserved) != maxReservations {
		t.Fatal("reservations are bounded")
	}
}

func TestPrivilegePolicyMatrix(t *testing.T) {
	strict := &guard{legacy: func() bool { return false }}
	legacy := &guard{legacy: func() bool { return true }}
	cases := []struct {
		g      *guard
		tier   string
		tool   string
		action string
		block  bool
	}{
		{strict, tierReadOnly, "edit", "", true}, {strict, tierReadOnly, "write", "", true}, {strict, tierReadOnly, "Bash", "", true},
		{strict, tierReadOnly, "powershell", "", true}, {strict, tierReadOnly, "read", "", false}, {strict, tierReadOnly, "GREP", "", false},
		{strict, tierReadOnly, "show_image", "", false}, {strict, tierReadOnly, "mcp", "", true}, {strict, tierReadOnly, "terminal_tools", "", true},
		{strict, tierReadOnly, "notify_human", "", true}, {strict, tierReadOnly, "agent_send", "", true},
		{strict, tierReadOnly, "schedule", "create", true}, {strict, tierReadOnly, "schedule", "run_now", true}, {strict, tierReadOnly, "schedule", "trust", true},
		{strict, tierReadOnly, "schedule", "list", false}, {strict, tierReadOnly, "schedule", "history", false}, {strict, tierReadOnly, "schedule", "", false},
		{legacy, tierReadOnly, "notify_human", "", false}, {legacy, tierReadOnly, "bash", "", true}, {legacy, tierReadOnly, "powershell", "", true},
		{legacy, tierReadOnly, "agent_request", "", true}, {legacy, tierReadOnly, "schedule", "cancel", true},
		{strict, tierSuggest, "edit", "", false}, {strict, tierSuggest, "write", "", false}, {strict, tierSuggest, "bash", "", true},
		{strict, tierSuggest, "powershell", "", true}, {strict, tierSuggest, "terminal_exec", "", true}, {strict, tierSuggest, "agent_send", "", true},
		{strict, tierSuggest, "schedule", "enable", true}, {strict, tierSuggest, "schedule", "list", false}, {strict, tierSuggest, "notify_human", "", false},
		{strict, tierMutate, "bash", "", false}, {strict, tierMutate, "schedule", "create", false}, {strict, "", "bash", "", false},
	}
	for _, tc := range cases {
		input := map[string]any{}
		if tc.action != "" {
			input["action"] = tc.action
		}
		got := tc.g.check(tc.tier, tc.tool, input)
		if got.block != tc.block {
			t.Errorf("%s/%s/%s: block=%v", tc.tier, tc.tool, tc.action, got.block)
		}
		if got.block && !strings.HasPrefix(got.reason, "[pig-schedule] blocked") {
			t.Errorf("reason %q", got.reason)
		}
	}
	if r := strict.check(tierReadOnly, "notify_human", nil); !strings.Contains(r.reason, "PIG_SCHEDULE_PRIVILEGE_MODE=legacy") {
		t.Fatal("strict block names the escape hatch")
	}
}

func TestProcessGroupExecutor(t *testing.T) {
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	start := time.Now()
	res, err := runProcessGroup(context.Background(), "bash", []string{"-lc", "printf 'start\\n'; sleep 30 & echo $! > " + pidfile + "; wait"}, dir, 500*time.Millisecond)
	if err != nil || !res.killed || res.code != -1 || res.stdout != "start\n" || time.Since(start) > 5*time.Second {
		t.Fatalf("%+v %v after %s", res, err, time.Since(start))
	}
	raw, _ := os.ReadFile(pidfile)
	var pid int
	fmt.Sscan(string(raw), &pid)
	deadline := time.Now().Add(3 * time.Second)
	for {
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil || strings.Fields(string(stat))[2] == "Z" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background child survived the timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	res, err = runProcessGroup(context.Background(), "bash", []string{"-lc", "echo out; echo err >&2; exit 3"}, dir, 5*time.Second)
	if err != nil || res.code != 3 || res.killed || res.stdout != "out\n" || res.stderr != "err\n" {
		t.Fatalf("%+v %v", res, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	if res, _ = runProcessGroup(ctx, "bash", []string{"-lc", "sleep 30"}, dir, time.Minute); !res.killed {
		t.Fatal("cancellation (shutdown) kills the job", res)
	}
	big, _ := runProcessGroup(context.Background(), "bash", []string{"-lc", "head -c 1000000 /dev/zero | tr '\\0' a; echo; echo END"}, dir, 10*time.Second)
	if len(big.stdout) > 2*captureLimit+10 || !strings.HasSuffix(big.stdout, "END\n") {
		t.Fatal("bounded head/tail capture", len(big.stdout))
	}
	if _, err := runProcessGroup(context.Background(), filepath.Join(dir, "missing-shell"), nil, dir, time.Second); err == nil {
		t.Fatal("missing shell is an error")
	}
}
