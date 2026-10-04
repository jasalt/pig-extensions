// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func call(t *testing.T, tl tools, s *fakeSession, args map[string]any) (string, map[string]any) {
	t.Helper()
	result := tl.execute(s, args)
	details, _ := result.Details.(map[string]any)
	return result.Content, details
}

func TestToolActions(t *testing.T) {
	h := newHarness(t)
	tl := tools{runner: h.runner, limiter: &rateLimiter{max: maxCreatesPerMinute}}
	s := h.session()
	text, _ := call(t, tl, s, map[string]any{"action": "list"})
	if !strings.HasPrefix(text, "No scheduled jobs.") {
		t.Fatal(text)
	}
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"action": "create"}, `Error: "name" is required for create`},
		{map[string]any{"action": "create", "name": strings.Repeat("n", 201), "prompt": "p", "every": "1h"}, "Error: name is too long (201 chars; max 200)"},
		{map[string]any{"action": "create", "name": "x", "every": "1h"}, `Error: kind=prompt requires "prompt"`},
		{map[string]any{"action": "create", "name": "x", "prompt": "p"}, `Error: Provide exactly one of "every"`},
		{map[string]any{"action": "create", "name": "x", "prompt": "p", "every": "1h", "dailyAt": "09:00"}, `Error: Provide exactly one of`},
		{map[string]any{"action": "create", "name": "x", "prompt": "p", "every": "1h", "maxRuns": 1.5}, "Error: maxRuns must be a positive integer"},
		{map[string]any{"action": "create", "name": "x", "kind": "shell", "every": "1h"}, `Error: kind=shell requires "command"`},
		{map[string]any{"action": "create", "name": "x", "prompt": "p", "every": "5s"}, `Error: Unrecognized schedule "every 5s"`},
		{map[string]any{"action": "frobnicate"}, "Unknown action: frobnicate"},
		{map[string]any{"action": "cancel"}, `Error: "id" is required for cancel`},
		{map[string]any{"action": "enable", "id": "nope"}, "Job nope not found."},
		{map[string]any{"action": "run_now"}, `Error: "id" is required for run_now`},
		{map[string]any{"action": "history"}, "No run history yet."},
	} {
		if text, _ := call(t, tl, s, tc.args); !strings.HasPrefix(text, tc.want) {
			t.Errorf("%v: %q", tc.args, text)
		}
	}
	text, details := call(t, tl, s, map[string]any{"action": "create", "name": "review", "prompt": "Check it", "every": "30m"})
	job := details["job"].(Job)
	if !strings.HasPrefix(text, "Created job "+job.ID+` "review" (every 30m, global).`) || !strings.Contains(text, "kind=prompt  tier=read_only  missedWindow=catch_up_one") ||
		!strings.Contains(text, "Next run: in 30m.") || len(s.custom) != 0 {
		t.Fatal(text, s.custom)
	}
	text, details = call(t, tl, s, map[string]any{"action": "create", "name": "poll\x1b[2J", "kind": "shell", "command": "make", "tier": "read_only", "wakeOn": "failure", "failurePrompt": "fix", "once": "10m"})
	shell := details["job"].(Job)
	if shell.Tier != tierMutate || shell.Schedule.Type != "once" || !strings.Contains(text, `command="make"  wakeOn=failure`) {
		t.Fatal(text)
	}
	if len(s.custom) != 1 || !strings.Contains(s.custom[0].Content.(string), "created shell (runs as mutate) job") || strings.Contains(s.custom[0].Content.(string), "\x1b") ||
		!strings.Contains(s.custom[0].Content.(string), "every future session, in any project") {
		t.Fatal("high-privilege create notice", s.custom)
	}
	text, _ = call(t, tl, s, map[string]any{"action": "list"})
	if !strings.Contains(text, "["+"on/global/shell/mutate]") || !strings.Contains(text, "command: make") || !strings.Contains(text, "prompt: Check it") {
		t.Fatal(text)
	}
	text, _ = call(t, tl, s, map[string]any{"action": "disable", "id": job.ID})
	if text != `Disabled job `+job.ID+` "review".` {
		t.Fatal(text)
	}
	text, _ = call(t, tl, s, map[string]any{"action": "run_now", "id": job.ID})
	if !strings.HasPrefix(text, "Delivered job "+job.ID) || len(s.sent) != 1 {
		t.Fatal("run_now fires a disabled job explicitly", text)
	}
	text, _ = call(t, tl, s, map[string]any{"action": "history", "limit": 500.0})
	if !strings.HasPrefix(text, "Run history (newest first):\n- ") || !strings.Contains(text, "delivered  review("+job.ID+")  kind=prompt  src=run_now  prompt") {
		t.Fatal(text)
	}
	text, _ = call(t, tl, s, map[string]any{"action": "cancel", "id": job.ID})
	if text != `Cancelled job `+job.ID+` "review".` {
		t.Fatal(text)
	}
	terminated := shell
	if _, err := h.store.terminate(terminated, h.cwd, "once", h.now); err != nil {
		t.Fatal(err)
	}
	if text, _ = call(t, tl, s, map[string]any{"action": "run_now", "id": shell.ID}); !strings.Contains(text, "is terminated (once)") {
		t.Fatal(text)
	}
	s.sendErr = func(int) error { return os.ErrClosed }
	failing, _ := h.store.create(newTestJob("fail", scopeGlobal, ""), h.cwd)
	if text, _ = call(t, tl, s, map[string]any{"action": "run_now", "id": failing.ID}); !strings.HasPrefix(text, `Failed to deliver job fail "n": `) {
		t.Fatal(text)
	}
}

func TestToolProjectTrustAndLimits(t *testing.T) {
	h := newHarness(t)
	tl := tools{runner: h.runner, limiter: &rateLimiter{max: maxCreatesPerMinute}}
	_ = os.Mkdir(filepath.Join(h.cwd, ".pig"), 0o700)
	scheduled := h.session()
	scheduled.turn = true
	text, _ := call(t, tl, scheduled, map[string]any{"action": "create", "name": "a", "prompt": "p", "every": "1h"})
	if !strings.Contains(text, ", project).") || h.runner.trust.isTrusted(h.cwd) {
		t.Fatal("a scheduled turn must not trust its own project", text)
	}
	text, _ = call(t, tl, scheduled, map[string]any{"action": "list"})
	if !strings.Contains(text, "[untrusted-project — will not auto-fire; schedule action=trust]") || !strings.Contains(text, "1 project job(s) are in an untrusted project") {
		t.Fatal(text)
	}
	user := h.session()
	call(t, tl, user, map[string]any{"action": "create", "name": "b", "prompt": "p", "every": "1h", "tier": "mutate"})
	if !h.runner.trust.isTrusted(h.cwd) || len(user.custom) != 1 || !strings.Contains(user.custom[0].Content.(string), "this project's future sessions") {
		t.Fatal("interactive project create trusts; mutate prompt warns", user.custom)
	}
	text, _ = call(t, tl, user, map[string]any{"action": "list"})
	if strings.Contains(text, "untrusted") {
		t.Fatal(text)
	}
	text, details := call(t, tl, user, map[string]any{"action": "trust"})
	if !strings.HasPrefix(text, "Trusted project "+h.cwd+". 2 project job(s)") || details["projectJobs"] != 2 {
		t.Fatal(text)
	}
	for range maxCreatesPerMinute - 2 {
		call(t, tl, user, map[string]any{"action": "create", "name": "c", "prompt": "p", "every": "1h", "scope": "global"})
	}
	if text, _ = call(t, tl, user, map[string]any{"action": "create", "name": "c", "prompt": "p", "every": "1h"}); text != "Error: create rate limit (10/min). Slow down." {
		t.Fatal(text)
	}
}
