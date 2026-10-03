package savelast

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func assistantEntry(content any) map[string]any {
	return map[string]any{
		"type": "message",
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
		},
	}
}

func TestLatestAssistantTextUsesLatestAssistantAndOnlyTextBlocks(t *testing.T) {
	entries := []map[string]any{
		assistantEntry("old response"),
		{"type": "message", "message": map[string]any{"role": "user", "content": "question"}},
		assistantEntry([]any{
			nil,
			42,
			"not a block",
			map[string]any{"type": "thinking", "text": "private"},
			map[string]any{"type": "tool-call", "text": "tool payload"},
			map[string]any{"type": "image", "text": "image payload"},
			map[string]any{"type": "text", "text": "  # Résumé 🐈\n"},
			map[string]any{"type": "text", "text": nil},
			map[string]any{"type": "text", "text": 123},
			map[string]any{"type": "text", "text": ""},
			map[string]any{"type": "text", "text": "Last paragraph.  "},
		}),
	}

	got, found := latestAssistantText(entries)
	if !found {
		t.Fatal("latestAssistantText did not find the assistant message")
	}
	want := "  # Résumé 🐈\n\n\nLast paragraph.  "
	if got != want {
		t.Fatalf("latestAssistantText() = %q, want %q", got, want)
	}
}

func TestLatestAssistantTextDoesNotFallBackWhenLatestAssistantIsTextless(t *testing.T) {
	entries := []map[string]any{
		assistantEntry("earlier text must not be saved"),
		assistantEntry([]any{
			map[string]any{"type": "thinking", "text": "private"},
			map[string]any{"type": "tool-call", "name": "bash"},
		}),
	}

	got, found := latestAssistantText(entries)
	if !found {
		t.Fatal("latestAssistantText did not find the textless assistant message")
	}
	if got != "" {
		t.Fatalf("latestAssistantText() = %q, want empty text", got)
	}
}

func TestLatestAssistantTextPreservesStringContent(t *testing.T) {
	want := " \nA plain response — 決定\t\r\n  "
	got, found := latestAssistantText([]map[string]any{assistantEntry(want)})
	if !found {
		t.Fatal("latestAssistantText did not find the assistant message")
	}
	if got != want {
		t.Fatalf("latestAssistantText() = %q, want %q", got, want)
	}
}

func TestLatestAssistantTextReportsAbsentAssistant(t *testing.T) {
	got, found := latestAssistantText([]map[string]any{
		{"type": "message", "message": map[string]any{"role": "user", "content": "question"}},
		{"type": "toolResult", "content": "output"},
	})
	if found {
		t.Fatalf("latestAssistantText found absent assistant with text %q", got)
	}
}

func TestResolveTargetUsesCommandCwdAndNormalizesPath(t *testing.T) {
	cwd := filepath.Join("/tmp", "project")
	got := resolveTarget(cwd, " \t../exports/unused/../résumé with spaces.md  ", time.UnixMilli(123))
	want := filepath.Join("/tmp", "exports", "résumé with spaces.md")
	if got != want {
		t.Fatalf("resolveTarget() = %q, want %q", got, want)
	}

	absolute := filepath.Join("/tmp", "absolute", "nested", "out.md")
	if got := resolveTarget(cwd, absolute, time.UnixMilli(456)); got != absolute {
		t.Fatalf("resolveTarget(absolute) = %q, want %q", got, absolute)
	}
}

func TestResolveTargetDefaultsToEpochMillisecondMarkdownFile(t *testing.T) {
	got := resolveTarget(filepath.Join("/tmp", "project"), " \t\n ", time.UnixMilli(123456789))
	want := filepath.Join("/tmp", "project", "123456789.md")
	if got != want {
		t.Fatalf("resolveTarget() = %q, want %q", got, want)
	}
}

func TestWriteTextFileCreatesParentsAndDoesNotAppendNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes", "résumé with spaces.md")
	want := "  # Résumé 🐈\nLast paragraph.  "

	if err := writeTextFile(path, want); err != nil {
		t.Fatalf("writeTextFile() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != want {
		t.Fatalf("written file = %q, want %q", got, want)
	}
}

func TestWriteTextFileOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.md")
	if err := os.WriteFile(path, []byte("longer old file contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeTextFile(path, "new"); err != nil {
		t.Fatalf("writeTextFile() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("written file = %q, want %q", got, "new")
	}
}
