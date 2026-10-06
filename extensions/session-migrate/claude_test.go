// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package sessionmigrate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRows(t *testing.T, path string) []object {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []object
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row object
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}
func fixture(t *testing.T, rows []object) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, row := range rows {
		if err := json.NewEncoder(f).Encode(row); err != nil {
			t.Fatal(err)
		}
	}
	return path
}
func msg(id, parent, role string, content any) object {
	return object{"type": role, "uuid": id, "parentUuid": parent, "timestamp": "2026-01-01T00:00:00Z", "sessionId": "source", "message": object{"role": role, "content": content}}
}

func TestNativeClaude(t *testing.T) {
	s, err := ReadClaude(context.Background(), "testdata/claude-native.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.Report.SourceSHA256 != "9103243215e8cfb960495d2e0097c2c6d5787fe6dc93e2166fad7c7ebe827c3c" {
		t.Fatal(s.Report)
	}
	for key, want := range map[string]int{"tool_calls": 3, "tool_results": 3, "images": 1} {
		if s.Report.Preserved[key] != want {
			t.Fatalf("%s: %+v", key, s.Report)
		}
	}
	if s.Report.Omitted["private_thinking"] != 3 || s.Report.Omitted["document"] != 1 {
		t.Fatal(s.Report)
	}
	dir := t.TempDir()
	out, err := WriteSession(context.Background(), s, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	rows := readRows(t, out.Path)
	if rows[0]["version"] != float64(3) || rows[0]["id"] == "source" {
		t.Fatal(rows[0])
	}
	parent := any(nil)
	for _, row := range rows[1:] {
		if row["parentId"] != parent {
			t.Fatal("broken tree")
		}
		parent = row["id"]
	}
	data, _ := os.ReadFile(out.Path)
	if strings.Contains(string(data), "\"thinking\"") || strings.Contains(string(data), "\"signature\"") || strings.Contains(string(data), "\"document\"") {
		t.Fatal("nonportable block leaked")
	}
	audit, _ := os.ReadFile(out.ManifestPath)
	if strings.Contains(string(audit), s.Title) {
		t.Fatal("manifest contains title")
	}
	for _, path := range []string{out.Path, out.ManifestPath} {
		st, _ := os.Stat(path)
		if st.Mode().Perm() != 0o600 {
			t.Fatal(st.Mode())
		}
	}
	out2, err := WriteSession(context.Background(), s, dir, dir)
	if err != nil || out2.Path == out.Path {
		t.Fatal(out2, err)
	}
}

func TestGraphOrderAndFork(t *testing.T) {
	rows := []object{msg("u", "", "user", "hello"), msg("r", "a", "user", []any{object{"type": "tool_result", "tool_use_id": "call", "content": "ok"}}), msg("a", "u", "assistant", []any{object{"type": "tool_use", "id": "call", "name": "Read", "input": object{"n": json.Number("9007199254740993")}}}), msg("fork", "u", "assistant", "EXCLUDED"), object{"type": "last-prompt", "leafUuid": "r"}}
	s, err := ReadClaude(context.Background(), fixture(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != 3 || obj(s.Entries[2]["message"])["role"] != "toolResult" || s.Report.Omitted["inactive_or_metadata_record"] != 1 {
		t.Fatal(s)
	}
	data, _ := json.Marshal(s.Entries)
	if strings.Contains(string(data), "EXCLUDED") || !strings.Contains(string(data), "9007199254740993") {
		t.Fatal(string(data))
	}
}

func TestRejectGraphsAndTools(t *testing.T) {
	cases := map[string][]object{
		"duplicate":  {msg("a", "", "user", "x"), msg("a", "", "assistant", "y")},
		"cycle":      {msg("a", "b", "user", "x"), msg("b", "a", "assistant", "y")},
		"missing":    {msg("a", "gone", "user", "x")},
		"leaf":       {msg("a", "", "user", "x"), object{"type": "last-prompt", "leafUuid": "gone"}},
		"orphan":     {msg("a", "", "user", []any{object{"type": "tool_result", "tool_use_id": "gone", "content": "x"}})},
		"unresolved": {msg("a", "", "assistant", []any{object{"type": "tool_use", "id": "c", "name": "Bash", "input": object{}}})},
	}
	side := msg("a", "", "user", "x")
	side["isSidechain"] = true
	cases["sidechain"] = []object{side}
	mixed := msg("b", "a", "assistant", "x")
	mixed["sessionId"] = "other"
	cases["mixed"] = []object{msg("a", "", "user", "x"), mixed}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadClaude(context.Background(), fixture(t, rows)); err == nil {
				t.Fatal("accepted invalid transcript")
			}
		})
	}
}

func TestCompactionAndImages(t *testing.T) {
	summary := msg("summary", "boundary", "user", "COMPACTED")
	summary["isCompactSummary"] = true
	rows := []object{msg("old", "", "user", "OLD"), object{"type": "system", "subtype": "compact_boundary", "uuid": "boundary", "logicalParentUuid": "old"}, summary, msg("new", "summary", "user", []any{object{"type": "image", "source": object{"type": "base64", "media_type": "image/png", "data": "aGVsbG8="}}, object{"type": "thinking", "thinking": "SECRET"}, object{"type": "document"}, object{"type": "image", "source": object{"type": "url", "url": "https://example.invalid"}}})}
	s, err := ReadClaude(context.Background(), fixture(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	if s.Report.Preserved["compactions"] != 1 || s.Report.Preserved["images"] != 1 || s.Report.Omitted["unsupported_image"] != 1 {
		t.Fatal(s.Report)
	}
	dir := t.TempDir()
	out, err := WriteSession(context.Background(), s, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range readRows(t, out.Path) {
		if row["type"] == "compaction" && row["firstKeptEntryId"] != row["id"] {
			t.Fatal(row)
		}
	}
}

func TestPreservedCompactionLoop(t *testing.T) {
	summary := msg("s", "b", "user", "summary")
	summary["isCompactSummary"] = true
	boundary := object{"type": "system", "subtype": "compact_boundary", "uuid": "b", "logicalParentUuid": "tail", "compactMetadata": object{"preservedSegment": object{"anchorUuid": "s", "headUuid": "head", "tailUuid": "tail"}, "preservedMessages": object{"allUuids": []any{"head", "tail"}}}}
	rows := []object{boundary, summary, msg("head", "s", "user", "head"), msg("tail", "head", "assistant", "tail")}
	if _, err := ReadClaude(context.Background(), fixture(t, rows)); err != nil {
		t.Fatal(err)
	}
	delete(boundary, "compactMetadata")
	if _, err := ReadClaude(context.Background(), fixture(t, rows)); err == nil {
		t.Fatal("accepted undeclared loop")
	}
}

func TestCancellationAndMalformed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadClaude(ctx, "testdata/claude-native.jsonl"); err != context.Canceled {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if _, err := WriteSession(ctx, &Transcript{Entries: []object{{"type": "message"}}}, dir, dir); err != context.Canceled {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 0 {
		t.Fatal(files)
	}
	for _, data := range []string{"null\n", "[]\n", "{", "{} {}\n", "{\"secret\": NaN}\n", "\xff"} {
		path := filepath.Join(dir, "malformed")
		os.WriteFile(path, []byte(data), 0o600)
		if _, err := ReadClaude(context.Background(), path); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
}
