// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// execResult is one finished shell job process.
type execResult struct {
	stdout, stderr string
	code           int
	killed         bool
}

// executor runs one shell job. The default runs it in its own process group
// so a timeout or shutdown kills the whole tree; PiG's host exec only kills
// the direct child, and a backgrounded grandchild then holds the call open.
type executor func(ctx context.Context, shell string, args []string, cwd string, timeout time.Duration) (execResult, error)

// captureLimit bounds retained output; only head and tail are kept, which is
// all truncateOutput reports.
const captureLimit = 64 * 1024

type headTail struct {
	mu   sync.Mutex
	head []byte
	tail []byte
	cut  bool
}

func (w *headTail) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	if room := captureLimit - len(w.head); room > 0 {
		take := min(room, len(p))
		w.head = append(w.head, p[:take]...)
		p = p[take:]
	}
	if len(p) > 0 {
		w.tail = append(w.tail, p...)
		if extra := len(w.tail) - captureLimit; extra > 0 {
			w.tail = w.tail[extra:]
			w.cut = true
		}
	}
	return n, nil
}

func (w *headTail) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cut {
		return string(w.head) + "\n…\n" + string(w.tail)
	}
	return string(w.head) + string(w.tail)
}

func runProcessGroup(ctx context.Context, shell string, args []string, cwd string, timeout time.Duration) (execResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, shell, args...)
	cmd.Dir = cwd
	cmd.Stdin = nil
	var stdout, stderr headTail
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		// Negative pid signals the whole process group.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	// Do not wait for orphaned pipe holders after the group is killed.
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	result := execResult{stdout: stdout.String(), stderr: stderr.String()}
	if runCtx.Err() != nil {
		// Reap any survivors that escaped into the group after the kill.
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		result.code, result.killed = -1, true
		return result, nil
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.code = 0
	case errors.As(err, &exitErr):
		result.code = exitErr.ExitCode()
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			result.code, result.killed = -1, true
		}
	case errors.Is(err, exec.ErrWaitDelay):
		result.code = cmd.ProcessState.ExitCode()
	default:
		return result, err
	}
	return result, nil
}
