// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
// Test-only native renderer probe, not the pins extension or an accepted route.
package renderprobe

import (
	"fmt"
	"math"
	"strings"
	"sync"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/MichaelKinsy/PiG/tui"
	"github.com/MichaelKinsy/PiG/tui/widthx"
)

const sample = "# Native heading\n\n**Strong** and [native link](https://example.invalid/pin).\n\n```go\nfunc main() {\n    fmt.Println(\"native code\")\n}\n```\n\n| Item | Value |\n| --- | --- |\n| Native | Table |\n\n" + "Scroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\nScroll line\n"

func markdownTheme(ui sdk.UITheme) *tui.MarkdownTheme {
	return &tui.MarkdownTheme{
		Heading: func(s string) string { return ui.Fg("mdHeading", s) },
		Link:    func(s string) string { return ui.Fg("mdLink", s) }, LinkUrl: func(s string) string { return ui.Fg("mdLinkUrl", s) },
		Code: func(s string) string { return ui.Fg("mdCode", s) }, CodeBlock: func(s string) string { return ui.Fg("mdCodeBlock", s) },
		CodeBlockBorder: func(s string) string { return ui.Fg("mdCodeBlockBorder", s) }, Quote: func(s string) string { return ui.Fg("mdQuote", s) },
		QuoteBorder: func(s string) string { return ui.Fg("mdQuoteBorder", s) }, Hr: func(s string) string { return ui.Fg("mdHr", s) },
		ListBullet: func(s string) string { return ui.Fg("mdListBullet", s) },
		Bold:       ui.Bold, Italic: ui.Italic, Strikethrough: ui.Strikethrough, Underline: ui.Underline,
		// The native public highlighter is the hypothesis under test. It uses
		// ActiveTheme(), not this per-widget UI theme; do not mutate that global.
		HighlightCode: tui.HighlightCode,
	}
}

type browser struct {
	mu       sync.Mutex
	ctx      sdk.Context
	scroll   int
	disposed bool
	reports  []map[string]any
}

func (b *browser) Render(width int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.render(width)
}
func (b *browser) render(width int) []string {
	if b.disposed {
		return nil
	}
	width = max(1, width)
	terminalHeight := b.ctx.Height()
	height := max(1, int(math.Floor(float64(terminalHeight)*.8)))
	theme := b.ctx.UITheme()
	md := tui.NewMarkdownWithOptions(sample, 0, 0, markdownTheme(theme), nil, nil)
	content := md.Render(width)
	chrome := 0
	if height >= 4 {
		chrome = 3
	}
	rows := height - chrome
	b.scroll = max(0, min(b.scroll, max(0, len(content)-rows)))
	out := []string{}
	if chrome != 0 {
		out = append(out, theme.Fg("accent", "PIN RENDER PROBE"))
	}
	end := min(len(content), b.scroll+rows)
	out = append(out, content[b.scroll:end]...)
	for len(out) < height-chrome+min(chrome, 1) {
		out = append(out, "")
	}
	if chrome != 0 {
		out = append(out, theme.Fg("dim", fmt.Sprintf("probe %d/%d", b.scroll+1, len(content))), theme.Fg("accent", "g/G top/bottom · q/Esc close"))
	}
	for i, line := range out {
		out[i] = widthx.TruncateToWidth(line, width, "", false)
	}
	if len(b.reports) < 128 {
		b.reports = append(b.reports, map[string]any{"width": width, "terminalHeight": terminalHeight, "latestHeight": b.ctx.Height(), "rows": len(out), "scroll": b.scroll, "lines": append([]string(nil), out...)})
	}
	return out
}
func (b *browser) HandleInput(data string) (sdk.RemoteComponentResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// Materialize metrics even if selection/input arrives before the first paint.
	b.render(max(1, b.ctx.Width()))
	switch data {
	case "q", "\x1b", "\r", "\n":
		return sdk.RemoteComponentResult{Done: true}, nil
	case "\x1b[A":
		b.scroll = max(0, b.scroll-1)
	case "\x1b[B":
		b.scroll++
	case "g":
		b.scroll = 0
	case "G":
		b.scroll = 1 << 30
	}
	return sdk.RemoteComponentResult{}, nil
}
func (b *browser) Dispose() { b.mu.Lock(); defer b.mu.Unlock(); b.disposed = true }

func Extension() *sdk.Extension {
	ext := sdk.New("pins-render-probe")
	ext.Command("render-probe", "Test-only native Markdown and full-width overlay probe", func(ctx sdk.Context, _ string) error {
		if ctx.Mode() != "tui" {
			ctx.Notify("render-probe requires a terminal", "warning")
			return nil
		}
		b := &browser{ctx: ctx}
		_, err := ctx.Custom(b, sdk.RemoteOverlayOptions{Overlay: true, WidthFraction: 1, HeightFraction: .8, OverlayOptions: &sdk.OverlayOptions{
			Width: sdk.OverlayPercent(100), MaxHeight: sdk.OverlayPercent(80), Anchor: "top-left", Margin: &sdk.OverlayMargin{All: new(int)},
		}})
		if err != nil {
			return err
		}
		b.mu.Lock()
		reports := append([]map[string]any(nil), b.reports...)
		b.mu.Unlock()
		return ctx.AppendEntry("pins-render-probe-frames", reports)
	})
	ext.Command("render-probe-facts", "Native toolkit state compared with host-resolved theme", func(ctx sdk.Context, _ string) error {
		ui := ctx.UITheme()
		keyColor, err := ui.GetFgAnsi("syntaxKeyword")
		if err != nil {
			return err
		}
		native := strings.Join(tui.HighlightCode("func main() {}", "go"), "\n")
		caps := tui.GetCapabilities()
		ctx.Notify(fmt.Sprintf("host theme=%s keyword=%q; child native keyword lines=%q; child capabilities=%+v", ui.Name, keyColor, native, caps), "info")
		return ctx.AppendEntry("pins-render-probe-facts", map[string]any{
			"hostTheme": ui.Name, "hostKeyword": keyColor, "childHighlight": native,
			"childCapabilities": map[string]any{"hyperlinks": caps.Hyperlinks, "trueColor": caps.TrueColor, "images": caps.Images},
			"markdown":          tui.NewMarkdownWithOptions(sample, 0, 0, markdownTheme(ui), nil, nil).Render(80),
		})
	})
	return ext
}
