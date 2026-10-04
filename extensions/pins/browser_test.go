// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

import (
	"fmt"
	"strings"
	"testing"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/MichaelKinsy/PiG/tui/widthx"
)

func plain(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimRight(widthx.StripAnsi(line), " ")
	}
	return out
}

func testPin(id int, text string) Pin {
	return Pin{ID: id, Label: fmt.Sprintf("Pin %d", id), Text: text, PinnedAt: 1}
}

func contents(prefix string, n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("%s%d", prefix, i)
	}
	return strings.Join(parts, "\n\n")
}

type size struct{ width, rows int }

func newTestBrowser(pins []Pin, initial int, s *size) *browser {
	return newBrowser(pins, initial, func() sdk.UITheme { return sdk.UITheme{} }, func() int { return s.rows }, func() int { return s.width })
}

func has(lines []string, f func(string) bool) bool {
	for _, line := range lines {
		if f(line) {
			return true
		}
	}
	return false
}

func TestOverlayGeometryAndLayoutInvariants(t *testing.T) {
	opts := overlayOptions()
	if !opts.Overlay || opts.OverlayOptions.Width.Value != 100 || !opts.OverlayOptions.Width.Percent ||
		opts.OverlayOptions.MaxHeight.Value != 80 || opts.OverlayOptions.Anchor != "top-left" || *opts.OverlayOptions.Margin.All != 0 {
		t.Fatalf("%+v", opts.OverlayOptions)
	}
	if viewportHeight(24) != 19 || viewportHeight(1) != 1 || viewportHeight(0) != 1 {
		t.Fatal("viewport height")
	}
	for height := 1; height < 80; height++ {
		for _, n := range []int{1, 2, 10, 40} {
			l := browserLayout(height, n)
			if l.header+l.help+l.position+2*l.gap+l.list+l.content != height || l.content <= 0 || l.list < 0 || l.list > min(n, 6) {
				t.Fatalf("height %d pins %d: %+v", height, n, l)
			}
		}
	}
}

func TestRendersFullWidthWithoutBorders(t *testing.T) {
	s := &size{140, 24}
	b := newTestBrowser([]Pin{testPin(1, strings.Repeat("x", 140))}, 0, s)
	lines := plain(b.Render(140))
	if len(lines) != 19 || !strings.HasPrefix(lines[0], "📌 1 pin") {
		t.Fatalf("%q", lines)
	}
	if !has(lines, func(l string) bool { return l == strings.Repeat("x", 140) }) || widthx.VisibleWidth(b.Render(140)[3]) != 140 {
		t.Fatal("content must use the full width")
	}
	for _, line := range lines {
		if strings.ContainsAny(line, "│╭╮╰╯├┤") || widthx.VisibleWidth(line) > 140 {
			t.Fatalf("border glyph or overflow: %q", line)
		}
	}
	two := plain(newTestBrowser([]Pin{testPin(1, "a"), testPin(2, "b")}, 1, s).Render(80))
	if two[0] != "📌 2 pins" || two[1] != "  #1 · Pin 1" || two[2] != "❯ #2 · Pin 2" {
		t.Fatalf("%q", two[:3])
	}
}

func TestScrollingSwitchingTopBottom(t *testing.T) {
	s := &size{80, 20}
	b := newTestBrowser([]Pin{testPin(1, contents("Alpha ", 40)), testPin(2, "Beta only")}, 0, s)
	render := func() []string { return plain(b.Render(80)) }
	key := func(data string) {
		if result, err := b.HandleInput(data); err != nil || result.Done {
			t.Fatalf("%q closed: %+v %v", data, result, err)
		}
	}
	starts := func(prefix string) bool {
		return has(render(), func(l string) bool { return strings.HasPrefix(l, prefix) })
	}
	contains := func(part string) bool { return has(render(), func(l string) bool { return strings.Contains(l, part) }) }
	if !starts("Alpha 0") {
		t.Fatal(render())
	}
	key("\x1b[B")
	if !contains("· 2–") {
		t.Fatal(render())
	}
	key("\x1b[A")
	if !contains("· 1–") {
		t.Fatal(render())
	}
	key("G")
	if !starts("Alpha 39") {
		t.Fatal(render())
	}
	key("\x1b[B") // clamped at bottom
	if !starts("Alpha 39") {
		t.Fatal(render())
	}
	key("g")
	if !starts("Alpha 0") {
		t.Fatal(render())
	}
	key("\x1b[6~")
	lines := render()
	if !starts("Beta only") || lines[len(lines)-1] != "#2 · all visible" {
		t.Fatal(lines)
	}
	key("\x1b[6~") // clamped at last pin
	if !starts("Beta only") {
		t.Fatal(render())
	}
	key("\x1b[5~")
	if !starts("Alpha 0") {
		t.Fatal(render())
	}
	// Kitty keyboard protocol (CSI u) arrows and paging.
	key("\x1b[1;1B")
	key("\x1b[6;1~")
	if !starts("Beta only") {
		t.Fatal(render())
	}
}

func TestResizeAndTinyTerminalsKeepContentReachable(t *testing.T) {
	s := &size{130, 24}
	b := newTestBrowser([]Pin{testPin(1, contents("Wide line here ", 80))}, 0, s)
	b.Render(130)
	_, _ = b.HandleInput("G")
	for _, dims := range []size{{70, 40}, {11, 8}, {1, 1}, {2, 2}, {4, 3}, {100, 24}} {
		*s = dims
		lines := b.Render(dims.width)
		if len(lines) != viewportHeight(dims.rows) {
			t.Fatalf("%+v: %d lines", dims, len(lines))
		}
		for _, line := range lines {
			if widthx.VisibleWidth(line) > dims.width {
				t.Fatalf("%+v overflow %q", dims, line)
			}
		}
	}
	_, _ = b.HandleInput("g")
	if !has(plain(b.Render(100)), func(l string) bool { return strings.HasPrefix(l, "Wide line here 0") }) {
		t.Fatal("top not reachable after resize")
	}
}

func TestMarkdownTableAndCodeUseNativeRenderer(t *testing.T) {
	s := &size{120, 50}
	b := newTestBrowser([]Pin{testPin(1, "# Title\n\n| A | B |\n|---|---|\n| yes | no |\n\n```go\nfunc main() {}\n```")}, 0, s)
	lines := plain(b.Render(120))
	for _, want := range []string{"Title", "yes", "func main() {}", "│"} {
		if !has(lines, func(l string) bool { return strings.Contains(l, want) }) {
			t.Fatalf("missing %q in %q", want, lines)
		}
	}
	if has(lines, func(l string) bool { return strings.Contains(l, "# Title") }) {
		t.Fatal("heading markup should be rendered")
	}
}

func TestCloseKeysEvenBeforeFirstFrame(t *testing.T) {
	for _, data := range []string{"q", "\x1b", "\r", "\x1b[113u", "\x1b[27u", "\x1b[13u"} {
		s := &size{80, 24}
		b := newTestBrowser([]Pin{testPin(1, "text")}, 0, s)
		result, err := b.HandleInput(data)
		if err != nil || !result.Done {
			t.Fatalf("%q: %+v %v", data, result, err)
		}
	}
	s := &size{80, 24}
	b := newTestBrowser([]Pin{testPin(1, contents("L", 60))}, 0, s)
	_, _ = b.HandleInput("G") // before the first frame
	if !has(plain(b.Render(80)), func(l string) bool { return l == "L59" }) {
		t.Fatal("G before first paint must reach the bottom")
	}
	b.Dispose()
	if lines := b.Render(80); lines != nil {
		t.Fatal("disposed browser renders nothing")
	}
	if result, _ := b.HandleInput("x"); !result.Done {
		t.Fatal("disposed browser closes")
	}
}

func TestSnapshotIsImmutableAndSelectionClamped(t *testing.T) {
	pins := []Pin{testPin(1, "one"), testPin(2, "two")}
	s := &size{80, 24}
	b := newTestBrowser(pins, 9, s)
	pins[1].Text = "changed"
	lines := plain(b.Render(80))
	if !has(lines, func(l string) bool { return l == "two" }) || !has(lines, func(l string) bool { return l == "❯ #2 · Pin 2" }) {
		t.Fatal(lines)
	}
}
