// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

const (
	fileLockStale   = 30 * time.Second
	fileLockRetries = 20
	fileLockBackoff = 25 * time.Millisecond
	jobLockStale    = 30 * time.Minute
)

type lockPayload struct {
	PID   int    `json:"pid"`
	Token string `json:"token"`
	At    string `json:"at"`
}

func newToken() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%d-%d-%s", os.Getpid(), time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

func tryCreate(path string, body []byte) bool {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return false
	}
	return true
}

func readLockPayload(path string) (lockPayload, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return lockPayload{}, false
	}
	var payload lockPayload
	if json.Unmarshal(raw, &payload) != nil || payload.Token == "" || payload.At == "" {
		return lockPayload{}, false
	}
	return payload, true
}

// releaseOwned removes path only if it still carries token, so a stale
// takeover victim cannot unlink the new owner's lock.
func releaseOwned(path, token string) {
	if payload, ok := readLockPayload(path); ok && payload.Token == token {
		_ = os.Remove(path)
	}
}

// claimStale renames a stale lock aside; only one racer wins the rename.
func claimStale(path, token string) {
	dead := path + ".dead." + token
	if os.Rename(path, dead) == nil {
		_ = os.Remove(dead)
	}
}

// storeError is a user-facing persistence error.
type storeError struct{ msg string }

func (e storeError) Error() string { return e.msg }

// withFileLock serializes read-modify-write of one JSON file across processes.
// Only a lock older than fileLockStale (by mtime) is taken over.
func withFileLock[T any](path string, fn func() (T, error)) (T, error) {
	var zero T
	lockPath := path + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return zero, err
	}
	token := newToken()
	body, _ := json.Marshal(lockPayload{PID: os.Getpid(), Token: token, At: isoTime(time.Now())})
	acquired := false
	for range fileLockRetries {
		if tryCreate(lockPath, body) {
			acquired = true
			break
		}
		if info, err := os.Stat(lockPath); err == nil && time.Since(info.ModTime()) > fileLockStale {
			claimStale(lockPath, token)
			if tryCreate(lockPath, body) {
				acquired = true
				break
			}
		}
		time.Sleep(fileLockBackoff)
	}
	if !acquired {
		return zero, storeError{fmt.Sprintf("Could not acquire store lock for %s (another session busy, or a lock newer than %ds).", filepath.Base(path), int(fileLockStale.Seconds()))}
	}
	defer releaseOwned(lockPath, token)
	return fn()
}

// jobLocks is single-flight delivery per job: in-process and via O_EXCL files.
type jobLocks struct {
	mu   sync.Mutex
	dir  string
	held map[string]string
	now  func() time.Time
}

var unsafeLockChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func newJobLocks(dir string, now func() time.Time) *jobLocks {
	return &jobLocks{dir: dir, held: map[string]string{}, now: now}
}

func (l *jobLocks) path(jobID string) string {
	return filepath.Join(l.dir, unsafeLockChars.ReplaceAllString(jobID, "_")+".lock")
}

// tryAcquire returns a release func, or nil if the job is already held.
func (l *jobLocks) tryAcquire(jobID string) func() {
	l.mu.Lock()
	if _, busy := l.held[jobID]; busy {
		l.mu.Unlock()
		return nil
	}
	token := newToken()
	l.held[jobID] = token
	l.mu.Unlock()
	fail := func() func() {
		l.mu.Lock()
		delete(l.held, jobID)
		l.mu.Unlock()
		return nil
	}
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return fail()
	}
	path := l.path(jobID)
	body, _ := json.Marshal(lockPayload{PID: os.Getpid(), Token: token, At: isoTime(l.now())})
	if !tryCreate(path, body) {
		existing, ok := readLockPayload(path)
		switch {
		case !ok:
			if !tryCreate(path, body) {
				return fail()
			}
		case !l.stale(existing):
			return fail()
		default:
			claimStale(path, token)
			if !tryCreate(path, body) {
				return fail()
			}
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			if l.held[jobID] == token {
				delete(l.held, jobID)
			}
			l.mu.Unlock()
			releaseOwned(path, token)
		})
	}
}

func (l *jobLocks) stale(payload lockPayload) bool {
	at, ok := parseISO(payload.At)
	return !ok || l.now().Sub(at) > jobLockStale
}

var errNotFound = errors.New("not found")
