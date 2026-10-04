// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

// Package schedule implements the schedule tool: recurring and one-shot
// agent prompts, shell checks, reminders and session messages for PiG.
//
// Scope and policy follow pungggi/pi-schedule v0.4.0 (commit
// ed5ea93f1a82ac7038fda34acbbfaa572472f9e3). Jobs run only while a PiG
// session with this extension is open; there is no daemon.
package schedule

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

//go:embed skills/schedule/SKILL.md
var skillText []byte

// fireOnReasons are session_start reasons that run an immediate wave. A
// process "startup" wave is deferred to the first idle tick instead: PiG gives
// extensions no supported signal that the CLI carried an initial prompt, and
// firing first could run ahead of that prompt.
var fireOnReasons = map[string]bool{"new": true, "resume": true}

// Extension constructs the scheduler. Construction has no effects: stores,
// timers and the skill file are touched only by session events and tool calls.
func Extension() *sdk.Extension {
	return newExtension(time.Now, defaultTick, os.Getenv)
}

func resolveShell(getenv func(string) string) string {
	if shell := getenv("PIG_SCHEDULE_SHELL"); shell != "" {
		return shell
	}
	return "bash"
}

type scheduler struct {
	mu      sync.Mutex
	runner  *runner
	tools   tools
	guard   *guard
	getenv  func(string) string
	now     func() time.Time
	tick    time.Duration
	stop    chan struct{}
	ticking sync.WaitGroup
	waves   sync.WaitGroup
}

// sdkHost adapts a host Context for the runner and tool.
type sdkHost struct {
	sdk.Context
	guard *guard
}

// scheduledTurn reports whether the current turn belongs to a scheduled
// delivery (including shell follow-ups at tier=mutate).
func (h sdkHost) scheduledTurn() bool {
	if h.guard.empty() {
		return false
	}
	branch, err := h.SessionManager().GetBranch(nil)
	if err != nil {
		return true // fail closed: never grant trust when ownership is unknown
	}
	text, ok := latestUserText(branch)
	if !ok {
		return false
	}
	_, scheduled := h.guard.tierFor(text)
	return scheduled
}

func newExtension(now func() time.Time, tick time.Duration, getenv func(string) string) *sdk.Extension {
	g := &guard{legacy: func() bool { return getenv("PIG_SCHEDULE_PRIVILEGE_MODE") == "legacy" }}
	s := &scheduler{guard: g, getenv: getenv, now: now, tick: tick}
	ext := sdk.New("schedule")

	ext.OnEvent(sdk.EventResourcesDiscover, func(ctx sdk.Context, _ map[string]any) (any, error) {
		dir := filepath.Join(newPaths(ctx.ConfigHome()).stateDir, "skills", "schedule")
		path := filepath.Join(dir, "SKILL.md")
		if current, err := os.ReadFile(path); err != nil || string(current) != string(skillText) {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, nil
			}
			if err := writeFileAtomic(path, skillText); err != nil {
				return nil, nil
			}
		}
		return map[string]any{"skillPaths": []string{dir}}, nil
	})

	ext.OnEvent(sdk.EventSessionStart, func(ctx sdk.Context, data map[string]any) (any, error) {
		reason, _ := data["reason"].(string)
		r := s.start(ctx)
		host := sdkHost{Context: ctx, guard: g}
		if fireOnReasons[reason] {
			s.waves.Add(1)
			go func() {
				defer s.waves.Done()
				safeWave(r, host, sourceSessionStart)
			}()
		}
		s.startTicker(r, host)
		return nil, nil
	})
	ext.OnEvent(sdk.EventSessionBeforeCompact, func(sdk.Context, map[string]any) (any, error) {
		if r := s.current(); r != nil {
			r.setCompacting(true)
		}
		return nil, nil
	})
	for _, name := range []string{sdk.EventSessionCompact, sdk.EventSessionCompactFailed} {
		ext.OnEvent(name, func(sdk.Context, map[string]any) (any, error) {
			if r := s.current(); r != nil {
				r.setCompacting(false)
			}
			return nil, nil
		})
	}
	ext.OnEvent(sdk.EventToolCall, func(ctx sdk.Context, data map[string]any) (any, error) {
		if g.empty() {
			return nil, nil
		}
		branch, err := ctx.SessionManager().GetBranch(nil)
		if err != nil {
			return map[string]any{"block": true, "terminate": true,
				"reason": labelPrefix + " blocked: could not read the session to check scheduled-turn privilege: " + err.Error()}, nil
		}
		text, ok := latestUserText(branch)
		if !ok {
			return nil, nil
		}
		tier, scheduled := g.tierFor(text)
		if !scheduled {
			return nil, nil
		}
		name, _ := data["toolName"].(string)
		input, _ := data["input"].(map[string]any)
		if result := g.check(tier, name, input); result.block {
			return map[string]any{"block": true, "reason": result.reason, "terminate": true}, nil
		}
		return nil, nil
	})
	ext.OnEvent(sdk.EventAgentSettled, func(ctx sdk.Context, _ map[string]any) (any, error) {
		if g.empty() {
			return nil, nil
		}
		branch, err := ctx.SessionManager().GetBranch(nil)
		if err == nil {
			g.settled(branch)
		}
		return nil, nil
	})
	ext.OnEvent(sdk.EventSessionShutdown, func(sdk.Context, map[string]any) (any, error) {
		s.shutdown()
		return nil, nil
	})
	ext.RegisterTool(sdk.ToolDefinition{
		Name:        "schedule",
		Label:       "Schedule",
		Description: toolDescription,
		Parameters:  toolParameters,
		Execute: func(ctx sdk.Context, args map[string]any) (any, error) {
			r := s.current()
			if r == nil {
				r = s.start(ctx)
			}
			return s.tools.execute(sdkHost{Context: ctx, guard: g}, args), nil
		},
	})
	return ext
}

// start binds a runner to the session's config root, replacing any earlier one.
func (s *scheduler) start(ctx sdk.Context) *runner {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runner != nil && !s.runner.closed() {
		return s.runner
	}
	p := newPaths(ctx.ConfigHome())
	r := newRunner(store{paths: p, now: s.now}, ledger{path: p.runsFile, maxBytes: maxLedgerBytes},
		newJobLocks(p.lockDir, s.now), trustStore{path: p.trustFile}, s.guard, s.now,
		func() string { return resolveShell(s.getenv) })
	r.tick = s.tick
	s.runner = r
	s.tools = tools{runner: r, limiter: &rateLimiter{max: maxCreatesPerMinute}}
	return r
}

func (s *scheduler) current() *runner {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runner
}

func (s *scheduler) startTicker(r *runner, host sdkHost) {
	s.mu.Lock()
	if s.stop != nil {
		close(s.stop)
	}
	stop := make(chan struct{})
	s.stop = stop
	s.mu.Unlock()
	s.ticking.Add(1)
	go func() {
		defer s.ticking.Done()
		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-r.life.Done():
				return
			case <-host.Done():
				return
			case <-ticker.C:
				safeWave(r, host, sourceTick)
			}
		}
	}()
}

// shutdown stops the ticker and cancels pending waits, then joins background
// work for a bounded time (a host shell call can outlive the session briefly).
func (s *scheduler) shutdown() {
	s.mu.Lock()
	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
	r := s.runner
	s.mu.Unlock()
	if r != nil {
		r.close()
	}
	done := make(chan struct{})
	go func() {
		s.ticking.Wait()
		s.waves.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

// safeWave runs an automatic wave on an extension-owned goroutine. The SDK
// recovers handler panics but not these; in a fused Piglet Binary an
// unrecovered panic would terminate PiG itself.
func safeWave(r *runner, host session, source string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.emitError(host, fmt.Sprintf("runner panic: %v", recovered), "panic")
		}
	}()
	_, _ = r.fireDue(host, source, nil)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + "." + newRunID() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
