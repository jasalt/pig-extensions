// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// paths are the scheduler's PiG locations.
type paths struct {
	stateDir   string // <config-root>/state/schedule
	globalFile string
	runsFile   string
	lockDir    string
	trustFile  string
}

func newPaths(configRoot string) paths {
	dir := filepath.Join(configRoot, "state", "schedule")
	return paths{stateDir: dir, globalFile: filepath.Join(dir, "schedules.json"), runsFile: filepath.Join(dir, "runs.jsonl"),
		lockDir: filepath.Join(dir, "locks"), trustFile: filepath.Join(dir, "trusted.json")}
}

func absPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

func projectFile(root string) string { return filepath.Join(absPath(root), ".pig", "schedule.json") }

// defaultScope is project when <cwd>/.pig exists (no upward walk), else global.
func defaultScope(cwd string) string {
	if info, err := os.Stat(filepath.Join(absPath(cwd), ".pig")); err == nil && info.IsDir() {
		return scopeProject
	}
	return scopeGlobal
}

func newJobID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

type storeFile struct {
	Version int   `json:"version"`
	Jobs    []Job `json:"jobs"`
}

// writeJSONAtomic replaces path through a same-directory temporary file.
func writeJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.%d.%s.tmp", path, os.Getpid(), newRunID())
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// quarantine moves a corrupt store aside and reports it; it never returns an
// empty store that a later write would silently overwrite.
func quarantine(path, reason string, now time.Time) error {
	stamp := strings.NewReplacer(":", "-", ".", "-").Replace(isoTime(now))
	moved := path + ".corrupt-" + stamp
	if os.Rename(path, moved) == nil {
		return storeError{fmt.Sprintf("Schedule store unreadable (%s): quarantined to %s. Inspect it, restore over %s, then retry (no restart needed).",
			reason, filepath.Base(moved), filepath.Base(path))}
	}
	return storeError{fmt.Sprintf("Schedule store unreadable (%s): could not quarantine %s. Fix permissions/disk and retry.", reason, filepath.Base(path))}
}

func readStore(path string, now time.Time) ([]Job, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, quarantine(path, "read failed: "+err.Error(), now)
	}
	if strings.TrimSpace(string(raw)) == "" {
		return nil, nil
	}
	var parsed any
	if json.Unmarshal(raw, &parsed) != nil {
		return nil, quarantine(path, "invalid JSON", now)
	}
	object, ok := parsed.(map[string]any)
	if !ok {
		return nil, quarantine(path, "not a store object", now)
	}
	if version, _ := object["version"].(float64); version != 1 {
		return nil, quarantine(path, fmt.Sprintf("unsupported version %s (expected 1)", jsString(object["version"])), now)
	}
	rows, ok := object["jobs"].([]any)
	if !ok {
		return nil, quarantine(path, "missing jobs array", now)
	}
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		fields, ok := row.(map[string]any)
		if !ok {
			return nil, quarantine(path, "jobs array contains invalid rows", now)
		}
		jobs = append(jobs, normalizeRow(fields))
	}
	return jobs, nil
}

func jsString(v any) string {
	if v == nil {
		return "undefined"
	}
	raw, _ := json.Marshal(v)
	return strings.Trim(string(raw), `"`)
}

func clampString(s string, limit int) string {
	if jsLength(s) <= limit {
		return s
	}
	return jsSlice(s, limit)
}

// normalizeRow coerces a foreign row: wrong field types fall back to
// defaults, oversized text is clamped, and an unreadable schedule is kept as
// an invalid spec that is never due.
func normalizeRow(f map[string]any) Job {
	str := func(key string) string { s, _ := f[key].(string); return s }
	optStr := func(key string) *string {
		if s, ok := f[key].(string); ok {
			return &s
		}
		return nil
	}
	num := func(key string) (float64, bool) {
		n, ok := f[key].(float64)
		return n, ok && !math.IsNaN(n) && !math.IsInf(n, 0)
	}
	name := str("name")
	if _, ok := f["name"].(string); !ok {
		name = "unnamed"
	}
	job := Job{
		ID:                 str("id"),
		Name:               clampString(name, maxNameChars),
		Prompt:             clampString(str("prompt"), maxPromptChars),
		Command:            clampString(str("command"), maxCommandChars),
		SuccessPrompt:      clampString(str("successPrompt"), maxPromptChars),
		FailurePrompt:      clampString(str("failurePrompt"), maxPromptChars),
		Scope:              str("scope"),
		ProjectPath:        str("projectPath"),
		Enabled:            f["enabled"] == true,
		CreatedAt:          str("createdAt"),
		UpdatedAt:          str("updatedAt"),
		LastRunAt:          optStr("lastRunAt"),
		NextRunAt:          str("nextRunAt"),
		LastStatus:         optStr("lastStatus"),
		LastError:          str("lastError"),
		LastIdempotencyKey: str("lastIdempotencyKey"),
		Terminated:         optStr("terminated"),
	}
	if action, err := normalizeKind(str("action")); err == nil {
		job.Action = action
	} else {
		job.Action = kindPrompt
	}
	if wake := str("wakeOn"); isWakeOn(wake) {
		job.WakeOn = wake
	}
	if n, ok := num("timeoutMs"); ok && n > 0 {
		job.TimeoutMs = int64(n)
	}
	if n, ok := num("maxRuns"); ok && n > 0 && n == math.Trunc(n) {
		job.MaxRuns = int(min(n, 1_000_000))
	}
	if n, ok := num("runCount"); ok && n >= 0 {
		job.RunCount = int(n)
	}
	job.MissedWindow = str("missedWindow")
	if job.MissedWindow != missedSkip {
		job.MissedWindow = missedCatchUp
	}
	job.Tier = str("tier")
	if job.Tier != tierSuggest && job.Tier != tierMutate {
		job.Tier = tierReadOnly
	}
	if job.Scope != scopeProject {
		job.Scope = scopeGlobal
	}
	if shell, ok := f["lastShell"].(map[string]any); ok {
		raw, _ := json.Marshal(shell)
		var result ShellResult
		if json.Unmarshal(raw, &result) == nil {
			job.LastShell = &result
		}
	}
	job.Schedule = decodeSpec(f["schedule"])
	return job
}

func decodeSpec(v any) Spec {
	f, ok := v.(map[string]any)
	if !ok {
		return Spec{}
	}
	num := func(key string) (int64, bool) {
		n, ok := f[key].(float64)
		return int64(n), ok && n == math.Trunc(n)
	}
	text, _ := f["type"].(string)
	switch text {
	case "interval":
		if ms, ok := num("everyMs"); ok && ms >= msMinute && ms <= 90*msDay {
			every, _ := f["every"].(string)
			return Spec{Type: text, EveryMs: ms, Every: every}
		}
	case "once":
		if ms, ok := num("delayMs"); ok && ms > 0 && ms <= maxOnce {
			delay, _ := f["delay"].(string)
			return Spec{Type: text, DelayMs: ms, Delay: delay}
		}
	case "daily":
		hour, hourOK := num("hour")
		minute, minuteOK := num("minute")
		if hourOK && minuteOK && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
			return Spec{Type: text, Hour: int(hour), Minute: int(minute), At: fmt.Sprintf("%02d:%02d", hour, minute)}
		}
	}
	return Spec{}
}

// store is the hybrid global + project schedule store.
type store struct {
	paths paths
	now   func() time.Time
}

func (s store) fileFor(job Job, cwd string) string {
	if job.Scope == scopeProject {
		root := job.ProjectPath
		if root == "" {
			root = cwd
		}
		return projectFile(root)
	}
	return s.paths.globalFile
}

// list returns global rows plus this cwd's project rows. Provenance comes from
// the file a row was read from, so a project file cannot relabel a row as
// global and bypass the trust gate.
func (s store) list(cwd string) ([]Job, error) {
	root := absPath(cwd)
	global, err := readStore(s.paths.globalFile, s.now())
	if err != nil {
		return nil, err
	}
	project, err := readStore(projectFile(root), s.now())
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(global)+len(project))
	for _, job := range global {
		job.Scope, job.ProjectPath = scopeGlobal, ""
		out = append(out, job)
	}
	for _, job := range project {
		if job.ProjectPath != "" && absPath(job.ProjectPath) != root {
			continue
		}
		job.Scope, job.ProjectPath = scopeProject, root
		out = append(out, job)
	}
	return out, nil
}

func (s store) get(id, cwd string) (*Job, error) {
	jobs, err := s.list(cwd)
	if err != nil {
		return nil, err
	}
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i], nil
		}
	}
	return nil, nil
}

func (s store) due(cwd string, now time.Time) ([]Job, error) {
	jobs, err := s.list(cwd)
	if err != nil {
		return nil, err
	}
	var out []Job
	for _, job := range jobs {
		if next, ok := parseISO(job.NextRunAt); ok && job.Enabled && job.Schedule.Type != "" && !next.After(now) {
			out = append(out, job)
		}
	}
	return out, nil
}

// update applies fn to the freshest copy of one row inside the file lock.
// It returns nil when the row no longer exists (never resurrecting it).
func (s store) update(job Job, cwd string, fn func(*Job)) (*Job, error) {
	path := s.fileFor(job, cwd)
	return withFileLock(path, func() (*Job, error) {
		jobs, err := readStore(path, s.now())
		if err != nil {
			return nil, err
		}
		for i := range jobs {
			if jobs[i].ID != job.ID {
				continue
			}
			fresh := jobs[i]
			if job.Scope == scopeProject {
				fresh.Scope, fresh.ProjectPath = scopeProject, absPath(s.projectRoot(job, cwd))
			}
			fn(&fresh)
			jobs[i] = fresh
			if err := writeJSONAtomic(path, storeFile{Version: 1, Jobs: jobs}); err != nil {
				return nil, err
			}
			return &fresh, nil
		}
		return nil, nil
	})
}

func (s store) projectRoot(job Job, cwd string) string {
	if job.ProjectPath != "" {
		return job.ProjectPath
	}
	return cwd
}

// create inserts a job; the per-scope cap is checked inside the insert lock.
func (s store) create(job Job, cwd string) (Job, error) {
	path := s.fileFor(job, cwd)
	return withFileLock(path, func() (Job, error) {
		jobs, err := readStore(path, s.now())
		if err != nil {
			return Job{}, err
		}
		if len(jobs) >= maxJobsPerScope {
			return Job{}, storeError{fmt.Sprintf("Job limit reached (%d per %s scope). Cancel unused jobs first.", maxJobsPerScope, job.Scope)}
		}
		jobs = append(jobs, job)
		return job, writeJSONAtomic(path, storeFile{Version: 1, Jobs: jobs})
	})
}

func (s store) remove(id, cwd string) (*Job, error) {
	for _, path := range []string{s.paths.globalFile, projectFile(cwd)} {
		removed, err := withFileLock(path, func() (*Job, error) {
			jobs, err := readStore(path, s.now())
			if err != nil {
				return nil, err
			}
			for i, job := range jobs {
				if job.ID == id {
					jobs = append(jobs[:i], jobs[i+1:]...)
					return &job, writeJSONAtomic(path, storeFile{Version: 1, Jobs: jobs})
				}
			}
			return nil, nil
		})
		if err != nil || removed != nil {
			return removed, err
		}
	}
	return nil, nil
}

// markAttempt records a fire/skip on the freshest row: it advances nextRunAt
// from the fresh schedule and keeps a concurrent disable.
func (s store) markAttempt(job Job, cwd string, at time.Time, status, errText, key string, advance bool, shell *ShellResult) (*Job, error) {
	return s.update(job, cwd, func(fresh *Job) {
		if advance {
			if next, ok := nextAfter(fresh.Schedule, at); ok {
				fresh.NextRunAt = isoTime(next)
			}
		}
		if status == statusOK || status == statusError {
			fresh.RunCount++
		}
		fresh.LastRunAt = strPtr(isoTime(at))
		fresh.LastStatus = strPtr(status)
		fresh.LastError = errText
		if key != "" {
			fresh.LastIdempotencyKey = key
		}
		fresh.UpdatedAt = isoTime(at)
		if shell != nil {
			fresh.LastShell = shell
		}
	})
}

func (s store) setEnabled(job Job, cwd string, enabled bool) (*Job, error) {
	return s.update(job, cwd, func(fresh *Job) {
		fresh.Enabled = enabled
		if enabled {
			fresh.Terminated = nil
		}
		fresh.UpdatedAt = isoTime(s.now())
	})
}

func (s store) terminate(job Job, cwd, reason string, at time.Time) (*Job, error) {
	return s.update(job, cwd, func(fresh *Job) {
		fresh.Enabled = false
		fresh.Terminated = strPtr(reason)
		fresh.UpdatedAt = isoTime(at)
	})
}
