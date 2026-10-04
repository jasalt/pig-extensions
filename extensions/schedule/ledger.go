// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxHistory       = 200
	maxLedgerBytes   = 5 * 1024 * 1024
	rotateKeepLines  = 1000
	defaultHistory   = 10
	maxHistoryResult = 50
)

func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ledger is the append-only JSONL run trail and secondary idempotency check.
type ledger struct {
	path     string
	maxBytes int64
}

// append is best-effort: a full disk must never block nextRunAt advancement.
func (l ledger) append(run Run) bool {
	line, err := json.Marshal(run)
	if err != nil {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return false
	}
	_, err = withFileLock(l.path, func() (struct{}, error) {
		file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return struct{}{}, err
		}
		_, writeErr := file.Write(append(line, '\n'))
		closeErr := file.Close()
		if writeErr != nil {
			return struct{}{}, writeErr
		}
		if closeErr != nil {
			return struct{}{}, closeErr
		}
		l.rotateLocked()
		return struct{}{}, nil
	})
	return err == nil
}

// rotateLocked keeps the newest lines, bounded by count and half the byte cap.
func (l ledger) rotateLocked() {
	info, err := os.Stat(l.path)
	if err != nil || info.Size() <= l.maxBytes {
		return
	}
	raw, err := os.ReadFile(l.path)
	if err != nil {
		return
	}
	lines := nonEmptyLines(string(raw))
	target := max(int64(1), l.maxBytes/2)
	var keep []string
	var size int64
	for i := len(lines) - 1; i >= 0 && len(keep) < rotateKeepLines; i-- {
		add := int64(len(lines[i]) + 1)
		if size+add > target {
			break
		}
		keep = append([]string{lines[i]}, keep...)
		size += add
	}
	var body strings.Builder
	for _, line := range keep {
		body.WriteString(line + "\n")
	}
	tmp := l.path + "." + newRunID() + ".tmp"
	if os.WriteFile(tmp, []byte(body.String()), 0o600) == nil {
		if os.Rename(tmp, l.path) != nil {
			_ = os.Remove(tmp)
		}
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// recent returns up to maxHistory runs, newest first; corrupt lines are skipped.
func (l ledger) recent() []Run {
	raw, err := os.ReadFile(l.path)
	if err != nil {
		return nil
	}
	lines := nonEmptyLines(string(raw))
	if len(lines) > maxHistory {
		lines = lines[len(lines)-maxHistory:]
	}
	runs := make([]Run, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		var run Run
		if json.Unmarshal([]byte(lines[i]), &run) == nil {
			runs = append(runs, run)
		}
	}
	return runs
}

func (l ledger) wasDelivered(key string) bool {
	for _, run := range l.recent() {
		if run.IdempotencyKey == key && run.Status == "delivered" {
			return true
		}
	}
	return false
}

func (l ledger) history(jobID string, limit int) []Run {
	var out []Run
	for _, run := range l.recent() {
		if jobID == "" || run.JobID == jobID {
			out = append(out, run)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}
