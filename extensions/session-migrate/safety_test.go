// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package sessionmigrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSourceResolution(t *testing.T) {
	cwd := t.TempDir()
	path, err := resolveSource(cwd, "with spaces.jsonl")
	if err != nil || path != filepath.Join(cwd, "with spaces.jsonl") {
		t.Fatal(path, err)
	}
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "12345678-1234-1234-1234-123456789abc"
	if _, err := resolveSource(cwd, id); err == nil {
		t.Fatal("accepted absent UUID")
	}
	for _, project := range []string{"one", "two"} {
		directory := filepath.Join(root, "projects", project)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		source := filepath.Join(directory, id+".jsonl")
		if err := os.WriteFile(source, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		path, err = resolveSource(cwd, id)
		if project == "one" && (err != nil || path != source) {
			t.Fatal(path, err)
		}
		if project == "two" && err == nil {
			t.Fatal("accepted ambiguous UUID")
		}
	}
}

func TestPortableResultOrder(t *testing.T) {
	report := Report{Preserved: map[string]int{}, Omitted: map[string]int{}}
	content := portableResult([]any{object{"type": "text", "text": ""}, object{"type": "image", "source": object{"type": "base64", "media_type": "image/png", "data": "aGVsbG8="}}, object{"type": "text", "text": "last"}, object{"type": "tool_reference", "tool_name": "do not expose"}}, &report)
	if len(content) != 3 || content[0]["text"] != "" || content[1]["type"] != "image" || content[2]["text"] != "last" || report.Omitted["tool_result_block"] != 1 {
		t.Fatal(content, report)
	}
}

func TestLatestTitleAndMixedGraph(t *testing.T) {
	rows := []object{msg("a", "", "user", "x"), object{"type": "ai-title", "aiTitle": "old"}, object{"type": "ai-title", "aiTitle": "new"}}
	s, err := ReadClaude(context.Background(), fixture(t, rows))
	if err != nil || s.Title != "new" {
		t.Fatal(s, err)
	}
	rows = append(rows, object{"type": "custom-title", "customTitle": "custom"}, object{"type": "ai-title", "aiTitle": "ignored"})
	s, err = ReadClaude(context.Background(), fixture(t, rows))
	if err != nil || s.Title != "custom" {
		t.Fatal(s, err)
	}
	for _, rows := range [][]object{
		{msg("a", "", "user", "x"), msg("", "", "assistant", "y")},
		{msg("a", "", "assistant", []any{object{"type": "tool_use", "id": "c", "name": "Read", "input": object{}}, object{"type": "tool_use", "id": "c", "name": "Read", "input": object{}}})},
	} {
		if _, err := ReadClaude(context.Background(), fixture(t, rows)); err == nil {
			t.Fatal("accepted invalid graph/tools")
		}
	}
	side := msg("s", "a", "assistant", "private")
	side["isSidechain"] = true
	rows = []object{msg("a", "", "user", "x"), side, object{"type": "last-prompt", "leafUuid": "s"}}
	if _, err := ReadClaude(context.Background(), fixture(t, rows)); err == nil {
		t.Fatal("accepted sidechain leaf")
	}
	// Legacy UUID-less transcripts remain supported without flattening mixed graphs.
	s, err = ReadClaude(context.Background(), fixture(t, []object{msg("", "", "user", "x"), msg("", "", "assistant", "y")}))
	if err != nil || len(s.Entries) != 2 {
		t.Fatal(s, err)
	}
}

func TestFileAndWriterFailures(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadClaude(context.Background(), dir); err == nil {
		t.Fatal("accepted directory")
	}
	pipe := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClaude(context.Background(), pipe); err == nil {
		t.Fatal("accepted FIFO")
	}
	path := filepath.Join(dir, "too-large.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := ReadClaude(context.Background(), path); err == nil {
		t.Fatal("accepted oversized file")
	}
	source := fixture(t, []object{msg("a", "", "user", "unchanged")})
	s, err := ReadClaude(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, "existing.jsonl")
	os.WriteFile(marker, []byte("KEEP"), 0o600)
	if _, err := WriteSession(context.Background(), s, "relative", dir); err == nil {
		t.Fatal("accepted relative directory")
	}
	if _, err := WriteSession(context.Background(), s, marker, dir); err == nil {
		t.Fatal("accepted file as directory")
	}
	// Serialization failure is returned, never published as a partial artifact.
	s.Entries = append(s.Entries, object{"type": "message", "unsupported": make(chan int)})
	if _, err := WriteSession(context.Background(), s, dir, dir); err == nil {
		t.Fatal("accepted unencodable draft")
	}
	kept, _ := os.ReadFile(marker)
	if string(kept) != "KEEP" {
		t.Fatal("existing file modified")
	}
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".migration-") || strings.HasSuffix(entry.Name(), ".migration.json") {
			t.Fatal("failure left artifacts")
		}
	}
}
