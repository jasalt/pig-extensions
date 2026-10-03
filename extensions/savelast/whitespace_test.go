package savelast

import (
	"path/filepath"
	"testing"
	"time"
)

func TestTrimMatchesOriginalECMAScriptWhitespace(t *testing.T) {
	if got := trim("\ufeff\t \u2028\u2029\u00a0"); got != "" {
		t.Fatalf("source whitespace: %q", got)
	}
	if got := trim("\u0085"); got != "\u0085" {
		t.Fatalf("NEL is not source whitespace: %q", got)
	}
	if got := resolveTarget("/tmp/project", "\ufeff notes/answer.md \ufeff", time.UnixMilli(123)); got != filepath.Join("/tmp/project", "notes/answer.md") {
		t.Fatal(got)
	}
	if got := resolveTarget("/tmp/project", "\ufeff", time.UnixMilli(123)); got != "/tmp/project/123.md" {
		t.Fatal(got)
	}
}
