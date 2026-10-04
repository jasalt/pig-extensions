// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package imgview

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestViewerUpstreamAndPrivacy(t *testing.T) {
	root := filepath.Join(t.TempDir(), "viewer")
	img := &resolvedImage{Bytes: pngMagic, MimeType: "image/png", SourceLabel: `test/<source>"&'.png`, Extension: "png"}
	now := time.UnixMilli(1700000000123)
	first, err := writeViewer(img, root, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeViewer(img, root, now)
	if err != nil || first == second {
		t.Fatalf("viewer names must be unique: %q %q %v", first, second, err)
	}
	if !regexp.MustCompile(`/imgview-1700000000123-[0-9a-f]{8}\.html$`).MatchString(first) {
		t.Fatalf("name: %s", first)
	}
	html, _ := os.ReadFile(first)
	for _, want := range []string{"data:image/png;base64,iVBORw0KGgoAAAAN", `test/&lt;source&gt;&quot;&amp;&#39;.png`, "12 bytes · image/png", `<meta charset="utf-8">`} {
		if !strings.Contains(string(html), want) {
			t.Errorf("viewer lacks %q", want)
		}
	}
	if strings.Contains(string(html), "<source>") {
		t.Error("label not escaped")
	}
	if info, _ := os.Stat(root); info.Mode().Perm() != 0o700 {
		t.Errorf("root mode %v", info.Mode().Perm())
	}
	if info, _ := os.Stat(first); info.Mode().Perm() != 0o600 {
		t.Errorf("viewer mode %v", info.Mode().Perm())
	}
	// A pre-existing permissive root is tightened.
	loose := filepath.Join(t.TempDir(), "loose")
	_ = os.Mkdir(loose, 0o777)
	if _, err := writeViewer(img, loose, now); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(loose); info.Mode().Perm() != 0o700 {
		t.Errorf("loose root mode %v", info.Mode().Perm())
	}
	// A file in place of the root is a write error, not a claimed viewer.
	blocked := filepath.Join(t.TempDir(), "file")
	_ = os.WriteFile(blocked, nil, 0o600)
	if path, err := writeViewer(img, blocked, now); err == nil || path != "" {
		t.Fatalf("blocked root: %q %v", path, err)
	}
}

func TestGroupThousands(t *testing.T) {
	for n, want := range map[int]string{0: "0", 12: "12", 999: "999", 1000: "1,000", 8388608: "8,388,608", -1234: "-1,234"} {
		if got := groupThousands(n); got != want {
			t.Errorf("%d: %q", n, got)
		}
	}
}

func TestProcessOpenerUsesArgvAndReportsMissingExecutable(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "argv")
	script := filepath.Join(dir, "fake-open")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+record+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	opener := &processOpener{command: script}
	target := filepath.Join(dir, "a b;$(touch pwned).html")
	command, args, err := opener.Open(target)
	if err != nil || command != script || len(args) != 1 || args[0] != target {
		t.Fatal(command, args, err)
	}
	opener.Wait(5 * time.Second)
	got, _ := os.ReadFile(record)
	if string(got) != target+"\n" {
		t.Fatalf("argv: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("target was shell-interpreted")
	}
	missing := &processOpener{command: filepath.Join(dir, "missing-opener")}
	if _, _, err := missing.Open(target); err == nil {
		t.Fatal("missing executable must be an error")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
}
