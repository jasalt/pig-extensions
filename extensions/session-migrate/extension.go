// Copyright (c) 2026 Jarkko Saltiola
// Copyright (c) 2026 xhluca
// SPDX-License-Identifier: MIT

// Package sessionmigrate imports Claude Code transcripts into native PiG sessions.
package sessionmigrate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

const usage = "Usage: /session-migrate <inspect|save|import> claude <JSONL path or UUID>"

var uuidPattern = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// Extension is an importable factory, with no constructor-side effects.
func Extension() *sdk.Extension {
	e := sdk.New("session-migrate")
	var mu sync.Mutex
	e.Command("session-migrate", usage, func(ctx sdk.Context, args string) error {
		// Concurrent commands must not race session replacement. Never wait on this
		// lock: a reentrant replacement event could otherwise deadlock the host.
		if !mu.TryLock() {
			return fmt.Errorf("session migration already in progress")
		}
		defer mu.Unlock()
		return run(ctx, args)
	})
	return e
}

func run(ctx sdk.Context, args string) error {
	action, rest, _ := strings.Cut(strings.TrimSpace(args), " ")
	format, source, _ := strings.Cut(strings.TrimSpace(rest), " ")
	source = strings.TrimSpace(source)
	if (action != "inspect" && action != "save" && action != "import") || format != "claude" || source == "" {
		return fmt.Errorf("%s; only Claude → PiG is supported", usage)
	}
	// The remaining argument is a literal path (spaces allowed); no shell runs.
	if len(source) >= 2 && ((source[0] == '"' && source[len(source)-1] == '"') || (source[0] == '\'' && source[len(source)-1] == '\'')) {
		source = source[1 : len(source)-1]
	}
	path, err := resolveSource(ctx.Cwd(), source)
	if err != nil {
		return err
	}
	work, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	exited := make(chan struct{})
	defer func() { close(done); <-exited }()
	go func() {
		defer close(exited)
		select {
		case <-ctx.Done():
			cancel()
		case <-done:
		}
	}()
	s, err := ReadClaude(work, path)
	if err != nil {
		return err
	}
	if action == "inspect" {
		report, _ := json.Marshal(s.Report)
		ctx.Notify(string(report), "info")
		return nil
	}
	idle, err := ctx.IsIdle()
	if err != nil {
		return err
	}
	if !idle {
		return fmt.Errorf("wait for PiG to be idle before importing")
	}
	dir, err := ctx.SessionManager().GetSessionDir()
	if err != nil {
		return err
	}
	if dir == "" {
		return fmt.Errorf("host did not supply a session directory; save/import require session persistence (do not use --no-session)")
	}
	result, err := WriteSession(work, s, dir, ctx.Cwd())
	if err != nil {
		return err
	}
	// Announce the durable path before replacement (which may cancel this context).
	ctx.Notify("Saved imported session: "+result.Path+"; manifest: "+result.ManifestPath, "info")
	if action == "save" {
		return nil
	}
	replacement, err := ctx.SwitchSession(result.Path, nil)
	if err != nil {
		return fmt.Errorf("session saved at %s, but switch failed: %w", result.Path, err)
	}
	if replacement.Cancelled {
		ctx.Notify("Session switch cancelled; imported file retained at "+result.Path, "warning")
	}
	return nil
}

func resolveSource(cwd, source string) (string, error) {
	if uuidPattern.MatchString(source) {
		root := os.Getenv("CLAUDE_CONFIG_DIR")
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			root = filepath.Join(home, ".claude")
		}
		matches, err := filepath.Glob(filepath.Join(root, "projects", "*", source+".jsonl"))
		if err != nil {
			return "", err
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("Claude UUID resolved to %d files; supply an explicit JSONL path", len(matches))
		}
		return matches[0], nil
	}
	if strings.HasPrefix(source, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		source = filepath.Join(home, source[2:])
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(cwd, source)
	}
	return filepath.Abs(source)
}
