// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const defaultTick = 30 * time.Second

// Hard caps against self-spam and backlog storms.
const (
	maxJobsPerScope         = 50
	maxFiresPerSessionStart = 5
	maxFiresPerTick         = 3
	maxCreatesPerMinute     = 10
	maxNameChars            = 200
	maxPromptChars          = 20_000
	maxCommandChars         = 10_000
)

// idempotencyKey identifies one due slot: same job + same planned nextRunAt.
func idempotencyKey(job Job) string { return job.ID + ":" + job.NextRunAt }

// graceFor is how late still counts as on time for the skip policy.
func graceFor(job Job, tick time.Duration) time.Duration {
	floor := 2 * tick
	var period int64 = -1
	switch job.Schedule.Type {
	case "interval":
		period = job.Schedule.EveryMs
	case "once":
		period = job.Schedule.DelayMs
	}
	if period >= 0 {
		pct := time.Duration(int64(math.Floor(float64(period)*0.25))) * time.Millisecond
		return min(max(floor, pct), 15*time.Minute)
	}
	return max(floor, time.Hour)
}

type decision struct {
	fire   bool
	reason string
	key    string
}

// decideDue chooses fire or skip for a due (non-forced) job.
func decideDue(job Job, now time.Time, tick time.Duration) decision {
	key := idempotencyKey(job)
	policy := job.MissedWindow
	if policy == "" {
		policy = missedCatchUp
	}
	planned, _ := parseISO(job.NextRunAt)
	overdue := max(0, now.Sub(planned))
	grace := graceFor(job, tick)
	if policy == missedSkip && overdue > grace {
		return decision{reason: fmt.Sprintf("missed_window_skip (overdue %dm > grace)", jsRound(float64(overdue.Milliseconds())/60_000)), key: key}
	}
	reason := "due"
	if policy == missedCatchUp && overdue > grace {
		reason = "catch_up_one"
	}
	return decision{fire: true, reason: reason, key: key}
}

func nextAfter(spec Spec, at time.Time) (time.Time, bool) { return computeNextRunAt(spec, at, false) }

func tierContract(tier string) string {
	switch tier {
	case tierSuggest:
		return strings.Join([]string{
			"PRIVILEGE: suggest",
			"- You may draft patches or proposals, but do NOT apply them, commit, push, or open PRs unless the user explicitly asks in this turn.",
			"- Prefer dry-run / report output.",
		}, "\n")
	case tierMutate:
		return strings.Join([]string{
			"PRIVILEGE: mutate",
			"- Workspace changes are allowed when necessary for this task.",
			"- Still prefer the smallest safe change; summarize every mutation.",
		}, "\n")
	}
	return strings.Join([]string{
		"PRIVILEGE: read_only",
		"- Do NOT modify files, commit, push, open PRs, install packages, or change config.",
		"- Investigate and report only. Prefer read/search/status tools.",
	}, "\n")
}

// rateLimiter is a sliding one-minute create limiter.
type rateLimiter struct {
	mu    sync.Mutex
	max   int
	times []time.Time
}

func (r *rateLimiter) take(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	start := now.Add(-time.Minute)
	kept := r.times[:0]
	for _, t := range r.times {
		if !t.Before(start) {
			kept = append(kept, t)
		}
	}
	r.times = kept
	if len(r.times) >= r.max {
		return false
	}
	r.times = append(r.times, now)
	return true
}

// jsLength is String.prototype.length (UTF-16 code units).
func jsLength(s string) int { return len(utf16.Encode([]rune(s))) }

// jsSlice keeps the first n UTF-16 units without splitting a surrogate pair.
func jsSlice(s string, n int) string {
	units := 0
	for i, r := range s {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > n {
			return s[:i]
		}
		units += width
	}
	return s
}
