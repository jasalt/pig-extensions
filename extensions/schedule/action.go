// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"fmt"
	"math"
	"strings"
)

const (
	defaultShellTimeoutMs = 60_000
	maxShellTimeoutMs     = 10 * 60_000
	maxShellOutputChars   = 8_000
	genericFollowUp       = "Review this scheduled shell command result and decide next steps."
)

// actionError is a user-facing create/validation error.
type actionError struct{ msg string }

func (e actionError) Error() string { return e.msg }

func isKind(v string) bool {
	return v == kindPrompt || v == kindShell || v == kindNotify || v == kindMessage
}

func isWakeOn(v string) bool {
	return v == "always" || v == "failure" || v == "success" || v == "never"
}

func normalizeKind(raw string) (string, error) {
	if raw == "" {
		return kindPrompt, nil
	}
	if !isKind(raw) {
		return "", actionError{fmt.Sprintf("Invalid kind %q. Use prompt | shell | notify | message.", raw)}
	}
	return raw, nil
}

func hasFollowUpText(job Job) bool {
	return strings.TrimSpace(job.Prompt) != "" || strings.TrimSpace(job.SuccessPrompt) != "" || strings.TrimSpace(job.FailurePrompt) != ""
}

// resolveWakeOn defaults to always when any follow-up text exists, else never.
func resolveWakeOn(job Job) string {
	if job.WakeOn != "" && isWakeOn(job.WakeOn) {
		return job.WakeOn
	}
	if hasFollowUpText(job) {
		return "always"
	}
	return "never"
}

func shellOK(r ShellResult) bool { return r.Code == 0 && !r.Killed }

func shouldWake(job Job, r ShellResult) bool {
	switch resolveWakeOn(job) {
	case "always":
		return true
	case "success":
		return shellOK(r)
	case "failure":
		return !shellOK(r)
	}
	return false
}

// followUpFor picks successPrompt | failurePrompt | prompt | generic text.
func followUpFor(job Job, r ShellResult) string {
	ok := shellOK(r)
	if s := strings.TrimSpace(job.SuccessPrompt); ok && s != "" {
		return s
	}
	if s := strings.TrimSpace(job.FailurePrompt); !ok && s != "" {
		return s
	}
	if s := strings.TrimSpace(job.Prompt); s != "" {
		return s
	}
	if resolveWakeOn(job) != "never" {
		return genericFollowUp
	}
	return ""
}

// clampTimeout validates a tool-provided timeout (nil = default).
func clampTimeout(raw *float64) (int64, error) {
	if raw == nil {
		return defaultShellTimeoutMs, nil
	}
	n := *raw
	if math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return 0, actionError{"timeoutMs must be a positive number"}
	}
	return min(jsRound(n), maxShellTimeoutMs), nil
}

// truncateOutput keeps head and tail of long output, measured in UTF-16 units.
func truncateOutput(s string) string {
	if jsLength(s) <= maxShellOutputChars {
		return s
	}
	head := maxShellOutputChars/2 - 2
	tail := maxShellOutputChars - head - 5
	units := utf16Units(s)
	return string(decodeUnits(units[:head])) + "\n…\n" + string(decodeUnits(units[len(units)-tail:]))
}

type createFields struct {
	kind, prompt, command, wakeOn, successPrompt, failurePrompt string
	timeoutMs                                                   *float64
}

type normalizedAction struct {
	kind, prompt, command, wakeOn, successPrompt, failurePrompt string
	timeoutMs                                                   int64
	forceMutate                                                 bool
}

func checkLength(label, v string, limit int) error {
	if n := jsLength(v); n > limit {
		return actionError{fmt.Sprintf("%s is too long (%d chars; max %d)", label, n, limit)}
	}
	return nil
}

// normalizeCreateAction validates kind-specific create fields.
func normalizeCreateAction(f createFields) (normalizedAction, error) {
	kind, err := normalizeKind(f.kind)
	if err != nil {
		return normalizedAction{}, err
	}
	if kind == kindShell {
		command := strings.TrimSpace(f.command)
		if command == "" {
			return normalizedAction{}, actionError{`kind=shell requires "command" (e.g. "npm test" or "glab pipeline view 123").`}
		}
		out := normalizedAction{kind: kind, command: command, prompt: strings.TrimSpace(f.prompt),
			successPrompt: strings.TrimSpace(f.successPrompt), failurePrompt: strings.TrimSpace(f.failurePrompt), forceMutate: true}
		for _, check := range []struct {
			label, value string
			limit        int
		}{{"command", command, maxCommandChars}, {"prompt", out.prompt, maxPromptChars}, {"successPrompt", out.successPrompt, maxPromptChars}, {"failurePrompt", out.failurePrompt, maxPromptChars}} {
			if err := checkLength(check.label, check.value, check.limit); err != nil {
				return normalizedAction{}, err
			}
		}
		if f.wakeOn != "" && !isWakeOn(f.wakeOn) {
			return normalizedAction{}, actionError{fmt.Sprintf("Invalid wakeOn %q. Use always | failure | success | never.", f.wakeOn)}
		}
		out.wakeOn = resolveWakeOn(Job{WakeOn: f.wakeOn, Prompt: out.prompt, SuccessPrompt: out.successPrompt, FailurePrompt: out.failurePrompt})
		if out.timeoutMs, err = clampTimeout(f.timeoutMs); err != nil {
			return normalizedAction{}, err
		}
		return out, nil
	}
	prompt := strings.TrimSpace(f.prompt)
	if prompt == "" {
		if kind == kindPrompt {
			return normalizedAction{}, actionError{`kind=prompt requires "prompt" (the isolated task text).`}
		}
		return normalizedAction{}, actionError{fmt.Sprintf(`kind=%s requires "prompt" (the %s text).`, kind, kind)}
	}
	if err := checkLength("prompt", prompt, maxPromptChars); err != nil {
		return normalizedAction{}, err
	}
	switch {
	case strings.TrimSpace(f.command) != "":
		return normalizedAction{}, actionError{fmt.Sprintf(`"command" is only valid for kind=shell (got kind=%s).`, kind)}
	case f.wakeOn != "":
		return normalizedAction{}, actionError{fmt.Sprintf(`"wakeOn" is only valid for kind=shell (got kind=%s).`, kind)}
	case f.successPrompt != "" || f.failurePrompt != "":
		return normalizedAction{}, actionError{`"successPrompt"/"failurePrompt" are only valid for kind=shell.`}
	case f.timeoutMs != nil:
		return normalizedAction{}, actionError{fmt.Sprintf(`"timeoutMs" is only valid for kind=shell (got kind=%s).`, kind)}
	}
	return normalizedAction{kind: kind, prompt: prompt}, nil
}

func payloadSummary(job Job) string {
	if job.Action == kindShell {
		return job.Command
	}
	return job.Prompt
}

// normalizeMaxRuns validates a positive integer (nil = unlimited).
func normalizeMaxRuns(raw *float64) (int, error) {
	if raw == nil {
		return 0, nil
	}
	n := *raw
	if n != math.Trunc(n) || n <= 0 || math.IsInf(n, 0) {
		return 0, actionError{"maxRuns must be a positive integer"}
	}
	return int(min(n, 1_000_000)), nil
}

// terminalReason reports whether a job should stop after runCount runs.
func terminalReason(job Job, runCount int) string {
	if job.Schedule.Type == "once" {
		return "once"
	}
	if job.MaxRuns > 0 && runCount >= job.MaxRuns {
		return "maxRuns"
	}
	return ""
}
