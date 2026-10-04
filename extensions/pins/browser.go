// Copyright (c) 2026 s4lv0
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

import (
	"fmt"
	"strconv"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/MichaelKinsy/PiG/tui"
	"github.com/MichaelKinsy/PiG/tui/widthx"
)

const browserKeys = "↑↓ scroll · PgUp/PgDn pin · g/G top/bottom · q/Esc/Enter close"

// overlayOptions is the borderless, full-width, top-anchored browser that
// takes 80% of the terminal height.
func overlayOptions() sdk.RemoteOverlayOptions {
	zero := 0
	return sdk.RemoteOverlayOptions{Overlay: true, OverlayOptions: &sdk.OverlayOptions{
		Width:     sdk.OverlayPercent(100),
		MaxHeight: sdk.OverlayPercent(80),
		Anchor:    "top-left",
		Margin:    &sdk.OverlayMargin{All: &zero},
	}}
}

func viewportHeight(rows int) int { return max(1, rows*4/5) }

// layout reserves content first on tiny terminals; normal chrome never clips
// the footer.
type layout struct{ header, help, position, gap, list, content int }

func browserLayout(height, pinCount int) layout {
	l := layout{}
	if height >= 3 {
		l.header = 1
	}
	if height >= 6 {
		l.help = 1
	}
	if height >= 4 {
		l.position = 1
	}
	if height >= 8 {
		l.gap = 1
	}
	fixed := l.header + l.help + l.position + 2*l.gap
	if height >= 5 {
		l.list = max(1, min(pinCount, 6, (height-fixed)/3))
	}
	l.content = height - fixed - l.list
	return l
}

// browser is an immutable pin snapshot shown in an owned overlay. Render and
// input may arrive on different goroutines, so all state is guarded.
type browser struct {
	mu       sync.Mutex
	pins     []Pin
	theme    func() sdk.UITheme
	rows     func() int
	width    func() int
	selected int
	scroll   int
	count    int
	content  int
	measured bool
	cache    map[string][]string
	disposed bool
}

func newBrowser(pins []Pin, initial int, theme func() sdk.UITheme, rows, width func() int) *browser {
	return &browser{
		pins:     append([]Pin(nil), pins...),
		selected: max(0, min(initial, len(pins)-1)),
		theme:    theme, rows: rows, width: width,
		cache: map[string][]string{},
	}
}

func (b *browser) Render(width int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.render(width)
}

func (b *browser) markdownLines(pin Pin, width int, theme sdk.UITheme) []string {
	key := strconv.Itoa(pin.ID) + ":" + strconv.Itoa(width) + ":" + theme.Name
	if lines, ok := b.cache[key]; ok {
		return lines
	}
	h := highlighter{theme: theme}
	md := tui.NewMarkdownWithOptions(pin.Text, 0, 0, &tui.MarkdownTheme{
		Heading:         func(s string) string { return theme.Fg("mdHeading", s) },
		Link:            func(s string) string { return theme.Fg("mdLink", s) },
		LinkUrl:         func(s string) string { return theme.Fg("mdLinkUrl", s) },
		Code:            func(s string) string { return theme.Fg("mdCode", s) },
		CodeBlock:       func(s string) string { return theme.Fg("mdCodeBlock", s) },
		CodeBlockBorder: func(s string) string { return theme.Fg("mdCodeBlockBorder", s) },
		Quote:           func(s string) string { return theme.Fg("mdQuote", s) },
		QuoteBorder:     func(s string) string { return theme.Fg("mdQuoteBorder", s) },
		Hr:              func(s string) string { return theme.Fg("mdHr", s) },
		ListBullet:      func(s string) string { return theme.Fg("mdListBullet", s) },
		Bold:            theme.Bold, Italic: theme.Italic, Strikethrough: theme.Strikethrough, Underline: theme.Underline,
		HighlightCode: h.Highlight,
	}, nil, nil)
	lines := md.Render(width)
	if len(b.cache) > 32 {
		clear(b.cache)
	}
	b.cache[key] = lines
	return lines
}

func (b *browser) render(width int) []string {
	if b.disposed || len(b.pins) == 0 {
		return nil
	}
	width = max(1, width)
	theme := b.theme()
	height := viewportHeight(b.rows())
	l := browserLayout(height, len(b.pins))
	out := make([]string, 0, height)
	if l.header == 1 {
		noun := "pins"
		if len(b.pins) == 1 {
			noun = "pin"
		}
		out = append(out, theme.Fg("accent", fmt.Sprintf("📌 %d %s", len(b.pins), noun)))
	}
	start := max(0, min(b.selected-l.list/2, len(b.pins)-l.list))
	for i := start; i < start+l.list; i++ {
		pin := b.pins[i]
		if i == b.selected {
			out = append(out, theme.Fg("accent", fmt.Sprintf("❯ #%d · %s", pin.ID, pin.Label)))
		} else {
			out = append(out, theme.Fg("muted", fmt.Sprintf("  #%d · %s", pin.ID, pin.Label)))
		}
	}
	for range l.gap {
		out = append(out, "")
	}
	pin := b.pins[b.selected]
	lines := b.markdownLines(pin, width, theme)
	b.count, b.content, b.measured = len(lines), l.content, true
	b.scroll = max(0, min(b.scroll, len(lines)-l.content))
	end := min(len(lines), b.scroll+l.content)
	out = append(out, lines[b.scroll:end]...)
	for range l.content - (end - b.scroll) {
		out = append(out, "")
	}
	for range l.gap {
		out = append(out, "")
	}
	if l.help == 1 {
		out = append(out, theme.Fg("accent", browserKeys))
	}
	if l.position == 1 {
		position := "all visible"
		if len(lines) > l.content {
			position = fmt.Sprintf("%d–%d/%d", b.scroll+1, end, len(lines))
		}
		out = append(out, theme.Fg("muted", fmt.Sprintf("#%d · %s", pin.ID, position)))
	}
	for i, line := range out {
		out[i] = widthx.TruncateToWidth(line, width, "", false)
	}
	return out
}

func (b *browser) maxScroll() int { return max(0, b.count-b.content) }

func (b *browser) HandleInput(data string) (sdk.RemoteComponentResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disposed {
		return sdk.RemoteComponentResult{Done: true}, nil
	}
	// Keys can arrive before the first frame; measure once.
	if !b.measured {
		b.render(b.width())
	}
	key, _ := tui.ParseKey(data)
	switch {
	case key == "escape" || key == "enter" || key == "q" || data == "q":
		return sdk.RemoteComponentResult{Done: true}, nil
	case key == "up":
		b.scroll = max(0, b.scroll-1)
	case key == "down":
		b.scroll = min(b.maxScroll(), b.scroll+1)
	case key == "pageUp":
		b.selected, b.scroll = max(0, b.selected-1), 0
	case key == "pageDown":
		b.selected, b.scroll = min(len(b.pins)-1, b.selected+1), 0
	case key == "g" || data == "g":
		b.scroll = 0
	case data == "G" || key == "shift+g":
		b.scroll = b.maxScroll()
	}
	return sdk.RemoteComponentResult{}, nil
}

func (b *browser) Dispose() {
	b.mu.Lock()
	b.disposed = true
	b.cache = nil
	b.mu.Unlock()
}
