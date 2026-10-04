// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"encoding/json"
	"time"
)

// Job kinds, tiers, policies and statuses, as pi-schedule v0.4.0.
const (
	kindPrompt  = "prompt"
	kindShell   = "shell"
	kindNotify  = "notify"
	kindMessage = "message"

	tierReadOnly = "read_only"
	tierSuggest  = "suggest"
	tierMutate   = "mutate"

	missedCatchUp = "catch_up_one"
	missedSkip    = "skip"

	scopeGlobal  = "global"
	scopeProject = "project"

	statusOK      = "ok"
	statusError   = "error"
	statusSkipped = "skipped"
	statusLocked  = "locked"

	sourceSessionStart = "session_start"
	sourceTick         = "tick"
	sourceRunNow       = "run_now"
)

// Spec is one schedule: interval, daily local wall-clock time, or one-shot.
// Type is empty for an unreadable foreign row, which is never due.
type Spec struct {
	Type    string // "interval" | "daily" | "once"
	EveryMs int64
	Every   string
	Hour    int
	Minute  int
	At      string
	DelayMs int64
	Delay   string
}

func (s Spec) MarshalJSON() ([]byte, error) {
	switch s.Type {
	case "interval":
		return json.Marshal(map[string]any{"type": s.Type, "everyMs": s.EveryMs, "every": s.Every})
	case "daily":
		return json.Marshal(map[string]any{"type": s.Type, "hour": s.Hour, "minute": s.Minute, "at": s.At})
	case "once":
		return json.Marshal(map[string]any{"type": s.Type, "delayMs": s.DelayMs, "delay": s.Delay})
	}
	return []byte("null"), nil
}

// ShellResult is a captured, truncated shell execution.
type ShellResult struct {
	OK        bool   `json:"ok"`
	Command   string `json:"command"`
	Cwd       string `json:"cwd"`
	TimeoutMs int64  `json:"timeoutMs"`
	Code      int    `json:"code"`
	Killed    bool   `json:"killed"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
}

// Job is one stored schedule row (store file JSON shape).
type Job struct {
	ID                 string       `json:"id"`
	Name               string       `json:"name"`
	Prompt             string       `json:"prompt"`
	Action             string       `json:"action"`
	Command            string       `json:"command,omitempty"`
	WakeOn             string       `json:"wakeOn,omitempty"`
	SuccessPrompt      string       `json:"successPrompt,omitempty"`
	FailurePrompt      string       `json:"failurePrompt,omitempty"`
	TimeoutMs          int64        `json:"timeoutMs,omitempty"`
	Schedule           Spec         `json:"schedule"`
	Scope              string       `json:"scope"`
	ProjectPath        string       `json:"projectPath,omitempty"`
	Enabled            bool         `json:"enabled"`
	MissedWindow       string       `json:"missedWindow"`
	Tier               string       `json:"tier"`
	MaxRuns            int          `json:"maxRuns,omitempty"`
	Terminated         *string      `json:"terminated"`
	CreatedAt          string       `json:"createdAt"`
	UpdatedAt          string       `json:"updatedAt"`
	LastRunAt          *string      `json:"lastRunAt"`
	NextRunAt          string       `json:"nextRunAt"`
	RunCount           int          `json:"runCount"`
	LastStatus         *string      `json:"lastStatus"`
	LastError          string       `json:"lastError,omitempty"`
	LastIdempotencyKey string       `json:"lastIdempotencyKey,omitempty"`
	LastShell          *ShellResult `json:"lastShell,omitempty"`
}

// Run is one append-only ledger line.
type Run struct {
	RunID          string `json:"runId"`
	JobID          string `json:"jobId"`
	JobName        string `json:"jobName"`
	Scope          string `json:"scope"`
	ProjectPath    string `json:"projectPath,omitempty"`
	IdempotencyKey string `json:"idempotencyKey"`
	Source         string `json:"source"`
	Status         string `json:"status"`
	StartedAt      string `json:"startedAt"`
	EndedAt        string `json:"endedAt"`
	Detail         string `json:"detail,omitempty"`
	Tier           string `json:"tier"`
	MissedWindow   string `json:"missedWindow"`
	Action         string `json:"action,omitempty"`
}

// isoTime formats as JavaScript Date.toISOString does.
func isoTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

func parseISO(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	return t, err == nil
}

func strPtr(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
