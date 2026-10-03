// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT AND ISC

// Package savelast implements the /savelast PiG extension.
//
// The behavior is adapted from atomdmac/pi-savelast (commit
// efb580c1e7f95e230c2c413021b6680abfa287c7). PiG deliberately reads the
// active session branch instead of the complete session entry list.
package savelast

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

const commandDescription = "Save the last agent message to a file (usage: /savelast [path])"

// Extension constructs the savelast extension.
func Extension() *sdk.Extension {
	ext := sdk.New("savelast")
	ext.Command("savelast", commandDescription, func(ctx sdk.Context, args string) error {
		entries, err := ctx.SessionManager().GetBranch(nil)
		if err != nil {
			ctx.Notify(fmt.Sprintf("Failed to read session: %v", err), "error")
			return nil
		}

		text, found := latestAssistantText(entries)
		if !found {
			ctx.Notify("No agent message found to save", "warning")
			return nil
		}
		if trim(text) == "" {
			ctx.Notify("Last agent message has no text content to save", "warning")
			return nil
		}

		target := resolveTarget(ctx.Cwd(), args, time.Now())
		if err := writeTextFile(target, text); err != nil {
			ctx.Notify(fmt.Sprintf("Failed to write file: %v", err), "error")
			return nil
		}
		ctx.Notify("Saved to: "+target, "info")
		return nil
	})
	return ext
}

// latestAssistantText returns the text extracted from the latest assistant
// message in entries. found remains true when that message has no usable text;
// callers must not fall back to an older assistant response.
func latestAssistantText(entries []map[string]any) (text string, found bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if entry["type"] != "message" {
			continue
		}
		message, ok := entry["message"].(map[string]any)
		if !ok || message["role"] != "assistant" {
			continue
		}
		content, present := message["content"]
		// Original savelast treats undefined (missing) content as no message;
		// null and other present-but-textless values use the textless warning.
		if !present {
			return "", false
		}
		return extractTextContent(content), true
	}
	return "", false
}

func extractTextContent(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		parts := make([]string, 0, len(value))
		for _, block := range value {
			part, ok := block.(map[string]any)
			if !ok || part["type"] != "text" {
				continue
			}
			text, ok := part["text"].(string)
			if ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

// trim matches modern ECMAScript String.trim: BOM is whitespace, NEL is not.
func trim(text string) string {
	return strings.TrimFunc(text, func(r rune) bool {
		return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
	})
}

func resolveTarget(cwd, args string, now time.Time) string {
	trimmed := trim(args)
	if trimmed == "" {
		trimmed = fmt.Sprintf("%d.md", now.UnixMilli())
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	if cwd == "" {
		absolute, err := filepath.Abs(trimmed)
		if err == nil {
			return filepath.Clean(absolute)
		}
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(filepath.Join(cwd, trimmed))
}

func writeTextFile(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o666)
}
