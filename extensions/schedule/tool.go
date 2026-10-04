// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

const toolDescription = "Manage scheduled agent tasks and actions (reviews, polls, shell checks, reminders). " +
	"Actions: create, list, cancel, enable, disable, run_now, history, trust. " +
	"Create kind: prompt (default) | shell | notify | message. " +
	`Schedules: every "30m"/"2h"/"1d", dailyAt "09:00", or once "10m". ` +
	"Defaults: tier=read_only (shell→mutate), missedWindow=catch_up_one. " +
	"Prompt jobs enter the current session as a new user message; they do not start a fresh conversation. " +
	"Due jobs fire on session new/resume, and while the session is open and idle (checked every 30s, including after startup). " +
	"Project-scope jobs only auto-fire in trusted projects (action=trust trusts the current project)."

func enumSchema(description string, values ...string) map[string]any {
	out := map[string]any{"type": "string", "enum": values}
	if description != "" {
		out["description"] = description
	}
	return out
}

func stringSchema(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

var toolParameters = sdk.Schema{
	"type":     "object",
	"required": []string{"action"},
	"properties": map[string]any{
		"action":        enumSchema("", "create", "list", "cancel", "enable", "disable", "run_now", "history", "trust"),
		"name":          stringSchema("Job name (create)"),
		"kind":          enumSchema("", kindPrompt, kindShell, kindNotify, kindMessage),
		"once":          stringSchema(`One-shot delay, e.g. "10m" or "30s" (xor with every/dailyAt)`),
		"maxRuns":       map[string]any{"type": "number", "description": "Max deliveries before the job auto-disables (default: unlimited)."},
		"prompt":        stringSchema("Task/reminder text (required for prompt/notify/message; optional shell follow-up)."),
		"command":       stringSchema(`Shell command for kind=shell (e.g. "npm test").`),
		"wakeOn":        enumSchema("", "always", "failure", "success", "never"),
		"successPrompt": stringSchema("Shell only: agent follow-up when command succeeds."),
		"failurePrompt": stringSchema("Shell only: agent follow-up when command fails."),
		"timeoutMs":     map[string]any{"type": "number", "description": "Shell only: exec timeout ms (default 60000, max 600000)."},
		"every":         stringSchema(`Interval, e.g. "30m", "2h", "1d" (create)`),
		"dailyAt":       stringSchema(`Daily local time "HH:MM" (create)`),
		"scope":         enumSchema("", scopeGlobal, scopeProject),
		"missedWindow":  enumSchema("", missedCatchUp, missedSkip),
		"tier":          enumSchema("", tierReadOnly, tierSuggest, tierMutate),
		"id":            stringSchema("Job id"),
		"limit":         map[string]any{"type": "number", "description": "history limit"},
	},
}

func textResult(text string, details map[string]any) sdk.ToolResult {
	return sdk.ToolResult{Content: text, Details: details}
}

func errorResult(text, code string) sdk.ToolResult {
	return textResult(text, map[string]any{"error": code})
}

func argString(args map[string]any, key string) string { s, _ := args[key].(string); return s }

func argNumber(args map[string]any, key string) *float64 {
	if n, ok := args[key].(float64); ok {
		return &n
	}
	return nil
}

// toolHost is what the tool needs beyond the runner session.
type toolHost interface {
	session
	scheduledTurn() bool
}

type tools struct {
	runner  *runner
	limiter *rateLimiter
}

func (t tools) execute(h toolHost, args map[string]any) sdk.ToolResult {
	result, err := t.dispatch(h, args)
	if err != nil {
		return textResult("Error: "+err.Error(), map[string]any{"error": err.Error()})
	}
	return result
}

func (t tools) dispatch(h toolHost, args map[string]any) (sdk.ToolResult, error) {
	cwd := h.Cwd()
	id := strings.TrimSpace(argString(args, "id"))
	switch action := argString(args, "action"); action {
	case "create":
		return t.create(h, args, cwd)
	case "list":
		return t.list(cwd)
	case "cancel":
		if id == "" {
			return errorResult(`Error: "id" is required for cancel`, "id_required"), nil
		}
		removed, err := t.runner.store.remove(id, cwd)
		if err != nil {
			return sdk.ToolResult{}, err
		}
		if removed == nil {
			return errorResult(fmt.Sprintf("Job %s not found.", argString(args, "id")), "not_found"), nil
		}
		return textResult(fmt.Sprintf("Cancelled job %s %q.", removed.ID, removed.Name), map[string]any{"job": removed}), nil
	case "enable", "disable":
		if id == "" {
			return errorResult(fmt.Sprintf(`Error: "id" is required for %s`, action), "id_required"), nil
		}
		job, err := t.runner.store.get(id, cwd)
		if err != nil {
			return sdk.ToolResult{}, err
		}
		var updated *Job
		if job != nil {
			if updated, err = t.runner.store.setEnabled(*job, cwd, action == "enable"); err != nil {
				return sdk.ToolResult{}, err
			}
		}
		if updated == nil {
			return errorResult(fmt.Sprintf("Job %s not found.", argString(args, "id")), "not_found"), nil
		}
		verb := "Disabled"
		if action == "enable" {
			verb = "Enabled"
		}
		return textResult(fmt.Sprintf("%s job %s %q.", verb, updated.ID, updated.Name), map[string]any{"job": updated}), nil
	case "run_now":
		return t.runNow(h, id, argString(args, "id"), cwd)
	case "history":
		limit := defaultHistory
		if n := argNumber(args, "limit"); n != nil && *n > 0 {
			limit = int(min(math.Floor(*n), maxHistoryResult))
		}
		runs := t.runner.ledger.history(id, max(limit, 1))
		if len(runs) == 0 {
			return textResult("No run history yet.", map[string]any{"runs": []Run{}}), nil
		}
		lines := []string{"Run history (newest first):"}
		for _, run := range runs {
			line := fmt.Sprintf("- %s  %s  %s(%s)", run.EndedAt, run.Status, run.JobName, run.JobID)
			if run.Action != "" {
				line += "  kind=" + run.Action
			}
			line += "  src=" + run.Source
			if run.Detail != "" {
				line += "  " + run.Detail
			}
			lines = append(lines, line+"  runId="+run.RunID)
		}
		return textResult(strings.Join(lines, "\n"), map[string]any{"runs": runs}), nil
	case "trust":
		if err := t.runner.trust.trust(cwd, t.runner.now()); err != nil {
			return sdk.ToolResult{}, err
		}
		jobs, err := t.runner.store.list(cwd)
		if err != nil {
			return sdk.ToolResult{}, err
		}
		count := 0
		for _, job := range jobs {
			if job.Scope == scopeProject {
				count++
			}
		}
		root := absPath(cwd)
		return textResult(fmt.Sprintf("Trusted project %s. %d project job(s) can now auto-fire when due. (Only trust projects you have inspected: .pig/schedule.json can contain shell jobs.)", root, count),
			map[string]any{"trusted": root, "projectJobs": count}), nil
	default:
		return errorResult(fmt.Sprintf("Unknown action: %s", action), "unknown_action"), nil
	}
}

func (t tools) create(h toolHost, args map[string]any, cwd string) (sdk.ToolResult, error) {
	now := t.runner.now()
	if !t.limiter.take(now) {
		return errorResult(fmt.Sprintf("Error: create rate limit (%d/min). Slow down.", maxCreatesPerMinute), "rate_limited"), nil
	}
	name := strings.TrimSpace(argString(args, "name"))
	if name == "" {
		return errorResult(`Error: "name" is required for create`, "name_required"), nil
	}
	if n := jsLength(name); n > maxNameChars {
		return errorResult(fmt.Sprintf("Error: name is too long (%d chars; max %d)", n, maxNameChars), "name_too_long"), nil
	}
	normalized, err := normalizeCreateAction(createFields{kind: argString(args, "kind"), prompt: argString(args, "prompt"),
		command: argString(args, "command"), wakeOn: argString(args, "wakeOn"), successPrompt: argString(args, "successPrompt"),
		failurePrompt: argString(args, "failurePrompt"), timeoutMs: argNumber(args, "timeoutMs")})
	if err != nil {
		return sdk.ToolResult{}, err
	}
	spec, err := scheduleFromParts(argString(args, "every"), argString(args, "dailyAt"), argString(args, "once"))
	if err != nil {
		return sdk.ToolResult{}, err
	}
	scope := argString(args, "scope")
	if scope != scopeGlobal && scope != scopeProject {
		scope = defaultScope(cwd)
	}
	missed := argString(args, "missedWindow")
	if missed != missedSkip {
		missed = missedCatchUp
	}
	tier := argString(args, "tier")
	if tier != tierSuggest && tier != tierMutate {
		tier = tierReadOnly
	}
	if normalized.forceMutate {
		tier = tierMutate
	}
	maxRuns, err := normalizeMaxRuns(argNumber(args, "maxRuns"))
	if err != nil {
		return sdk.ToolResult{}, err
	}
	next, _ := computeNextRunAt(spec, now, spec.Type == "daily")
	job := Job{ID: newJobID(), Name: name, Prompt: normalized.prompt, Action: normalized.kind, Command: normalized.command,
		WakeOn: normalized.wakeOn, SuccessPrompt: normalized.successPrompt, FailurePrompt: normalized.failurePrompt,
		TimeoutMs: normalized.timeoutMs, MaxRuns: maxRuns, Schedule: spec, Scope: scope, Enabled: true,
		MissedWindow: missed, Tier: tier, CreatedAt: isoTime(now), UpdatedAt: isoTime(now), NextRunAt: isoTime(next)}
	if scope == scopeProject {
		job.ProjectPath = absPath(cwd)
	}
	if job, err = t.runner.store.create(job, cwd); err != nil {
		return sdk.ToolResult{}, err
	}
	// An explicit create in this project trusts it; a scheduled turn must
	// never unlock its own project's gate.
	if scope == scopeProject && !h.scheduledTurn() {
		if err := t.runner.trust.trust(cwd, now); err != nil {
			return sdk.ToolResult{}, err
		}
	}
	shellBits := ""
	if job.Action == kindShell {
		command, _ := json.Marshal(job.Command)
		shellBits = fmt.Sprintf("  command=%s  wakeOn=%s", command, job.WakeOn)
	}
	t.notifyHighPrivilege(h, job)
	return textResult(strings.Join([]string{
		fmt.Sprintf("Created job %s %q (%s, %s).", job.ID, job.Name, formatSchedule(job.Schedule), job.Scope),
		fmt.Sprintf("kind=%s  tier=%s  missedWindow=%s%s", job.Action, job.Tier, job.MissedWindow, shellBits),
		fmt.Sprintf("Next run: %s.", formatRelative(job.NextRunAt, t.runner.now())),
		fmt.Sprintf("Use schedule action=run_now id=%s to fire immediately.", job.ID),
	}, "\n"), map[string]any{"job": job}), nil
}

// notifyHighPrivilege surfaces unattended shell/mutate jobs at create time.
func (t tools) notifyHighPrivilege(h toolHost, job Job) {
	isShell := job.Action == kindShell
	if !isShell && job.Tier != tierMutate {
		return
	}
	where := "this project's future sessions"
	if job.Scope == scopeGlobal {
		where = "every future session, in any project"
	}
	what := "prompt (tier=mutate)"
	command := ""
	if isShell {
		what = "shell (runs as mutate)"
		raw, _ := json.Marshal(job.Command)
		command = " command=" + oneLine(string(raw))
	}
	msg := fmt.Sprintf("%s created %s job %q — it will fire unattended in %s.%s If you did not expect this, cancel it: schedule action=cancel id=%s",
		labelPrefix, what, oneLine(job.Name), where, command, job.ID)
	h.Notify(msg, "warning")
	_ = h.SendCustomMessage(sdk.CustomMessage{CustomType: messageType, Content: msg, Display: true,
		Details: map[string]any{"jobId": job.ID, "kind": "create-notice", "scope": job.Scope, "tier": job.Tier}},
		sdk.SendMessageOptions{TriggerTurn: sdk.Bool(false)})
}

func truncateLine(s string, n int) string {
	one := jsTrim(jsSpaces.ReplaceAllString(s, " "))
	if jsLength(one) <= n {
		return one
	}
	return jsSlice(one, n-1) + "…"
}

func summarize(job Job, now time.Time) string {
	state := "off"
	switch {
	case job.Terminated != nil:
		state = "off/terminated:" + *job.Terminated
	case job.Enabled:
		state = "on"
	}
	last := "never"
	if job.LastRunAt != nil {
		last = formatRelative(*job.LastRunAt, now)
	}
	wake, runs, shell := "", "", ""
	if job.Action == kindShell && job.WakeOn != "" {
		wake = "  wakeOn: " + job.WakeOn
	}
	if job.MaxRuns > 0 {
		runs = fmt.Sprintf("  runs: %d/%d", job.RunCount, job.MaxRuns)
	}
	if job.LastShell != nil {
		shell = fmt.Sprintf("  lastExit=%d", job.LastShell.Code)
		if job.LastShell.Killed {
			shell += " killed"
		}
		if !job.LastShell.OK {
			shell += " (failed)"
		}
	}
	status := ""
	if job.LastStatus != nil {
		status = "  lastStatus: " + *job.LastStatus
	}
	label := "prompt"
	if job.Action == kindShell {
		label = "command"
	}
	return strings.Join([]string{
		fmt.Sprintf("- %s  %s  [%s/%s/%s/%s]", job.ID, oneLine(job.Name), state, job.Scope, job.Action, job.Tier),
		fmt.Sprintf("  schedule: %s  missedWindow: %s%s", formatSchedule(job.Schedule), job.MissedWindow, wake),
		fmt.Sprintf("  next: %s  last: %s  runs: %d%s%s%s", formatRelative(job.NextRunAt, now), last, job.RunCount, runs, status, shell),
		fmt.Sprintf("  %s: %s", label, truncateLine(stripControlChars(payloadSummary(job)), 120)),
	}, "\n")
}

func (t tools) list(cwd string) (sdk.ToolResult, error) {
	jobs, err := t.runner.store.list(cwd)
	if err != nil {
		return sdk.ToolResult{}, err
	}
	if len(jobs) == 0 {
		return textResult("No scheduled jobs. Create one with action=create, name, kind (optional), prompt or command, and every, dailyAt or once.", map[string]any{"jobs": []Job{}}), nil
	}
	now := t.runner.now()
	body := []string{"Scheduled jobs:"}
	untrusted := 0
	for _, job := range jobs {
		line := summarize(job, now)
		if job.Scope == scopeProject && !t.runner.eligible(job, cwd) {
			untrusted++
			line += "\n  [untrusted-project — will not auto-fire; schedule action=trust]"
		}
		body = append(body, line)
	}
	if untrusted > 0 {
		body = append(body, fmt.Sprintf("\n%d project job(s) are in an untrusted project: they never auto-fire. Inspect .pig/schedule.json first, then run schedule action=trust to allow auto-fire.", untrusted))
	}
	return textResult(strings.Join(body, "\n"), map[string]any{"jobs": jobs}), nil
}

func (t tools) runNow(h toolHost, id, rawID, cwd string) (sdk.ToolResult, error) {
	if id == "" {
		return errorResult(`Error: "id" is required for run_now`, "id_required"), nil
	}
	job, err := t.runner.store.get(id, cwd)
	if err != nil {
		return sdk.ToolResult{}, err
	}
	if job == nil {
		return errorResult(fmt.Sprintf("Job %s not found.", rawID), "not_found"), nil
	}
	if job.Terminated != nil {
		return textResult(fmt.Sprintf("Job %s %q is terminated (%s). Cancel and recreate to run again.", job.ID, job.Name, *job.Terminated),
			map[string]any{"error": "terminated", "job": job}), nil
	}
	results, err := t.runner.fireDue(h, sourceRunNow, []string{job.ID})
	if err != nil {
		var se storeError
		if errors.As(err, &se) {
			return sdk.ToolResult{}, err
		}
		return sdk.ToolResult{}, err
	}
	if len(results) == 0 {
		return textResult(fmt.Sprintf("Did not fire job %s %q: runner returned no result (another session may hold its lock, or the job became unavailable). Check schedule action=history id=%s.", job.ID, job.Name, job.ID),
			map[string]any{"error": "not_fired", "job": job}), nil
	}
	updated := results[0]
	switch deref(updated.LastStatus) {
	case statusOK:
		shell := ""
		if updated.LastShell != nil {
			shell = fmt.Sprintf(" shell exit=%d.", updated.LastShell.Code)
		}
		return textResult(fmt.Sprintf("Delivered job %s %q (kind=%s, tier=%s).%s Check schedule action=history id=%s.", updated.ID, updated.Name, updated.Action, updated.Tier, shell, updated.ID),
			map[string]any{"job": updated, "status": statusOK}), nil
	case statusError:
		reason := updated.LastError
		if reason == "" {
			reason = "unknown error"
		}
		return textResult(fmt.Sprintf("Failed to deliver job %s %q: %s", updated.ID, updated.Name, reason),
			map[string]any{"job": updated, "status": statusError, "error": updated.LastError}), nil
	case statusSkipped:
		reason := updated.LastError
		if reason == "" {
			reason = "policy"
		}
		return textResult(fmt.Sprintf("Job %s %q was skipped: %s", updated.ID, updated.Name, reason),
			map[string]any{"job": updated, "status": statusSkipped, "error": updated.LastError}), nil
	}
	return textResult(fmt.Sprintf("Job %s %q ended with status=%s. Check schedule action=history id=%s.", updated.ID, updated.Name, deref(updated.LastStatus), updated.ID),
		map[string]any{"job": updated, "status": deref(updated.LastStatus)}), nil
}
