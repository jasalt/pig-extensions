// Copyright (c) 2026 Gregory Johnson
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

// Package imgview implements the show_image tool and /imgcat, /imgshow and
// /imgboth commands for PiG.
//
// The behavior is adapted from gregjohnso/pi-imgview (commit
// 17b568e8e3b70d009adb8ac090d2b3280f065f11). Inline tool images use PiG's
// native tool-result rendering; slash-command images use PiG's public TUI
// Image component.
package imgview

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
	"github.com/MichaelKinsy/PiG/tui"
	"github.com/MichaelKinsy/PiG/tui/widthx"
)

// softMaxBytes is the warning-only per-image size note, as upstream.
const softMaxBytes = 8 * 1024 * 1024

// frameHeadroom reserves room for JSON framing, text and details beside the
// base64 image inside one extension wire frame.
const frameHeadroom = 1 << 20

const (
	modeTerminal = "terminal"
	modeBrowser  = "browser"
	modeBoth     = "both"
	messageType  = "imgview-image"
)

const toolDescription = "Display an image to the user. Renders inline in supported terminals and/or opens it in the user's default browser. Use this whenever you want the user to actually see an image — a screenshot, a generated diagram, a file from disk, an image at a URL."

var toolGuidelines = []string{
	"Use show_image when the user asks to view, see, or display an image, screenshot, plot, or diagram.",
	"Default show_image to mode='terminal'. Do NOT use mode='browser' or mode='both' unless the user explicitly asks to open the image in a browser (or asks to zoom into a large/high-resolution image where inline rendering would be too small to be useful). Browser mode launches an external window and is disruptive — never pick it on your own initiative.",
	"Pass show_image `source` as the literal path or URL the user gave you; do not paraphrase. For local files, ~ and relative paths are fine.",
	"Add a short show_image `caption` when the image's relevance isn't obvious from context (e.g. 'PR diff screenshot' or 'matplotlib output').",
}

var toolParameters = sdk.Schema{
	"type":     "object",
	"required": []string{"source"},
	"properties": map[string]any{
		"source": map[string]any{
			"type":        "string",
			"minLength":   1,
			"description": "Path to a local image file (absolute, relative to cwd, or starting with ~), an http(s):// URL, or a data: URI.",
		},
		"mode": map[string]any{
			"type":        "string",
			"enum":        []string{modeTerminal, modeBrowser, modeBoth},
			"description": "How to display the image. 'terminal' (default, strongly preferred) renders inline in supported terminals (iTerm2, Kitty, WezTerm, Ghostty) and is non-disruptive. 'browser' opens an external browser window — only use when the user explicitly asks for a browser view, or when they need to zoom into a large image. 'both' does both and should likewise be reserved for explicit user requests.",
		},
		"caption": map[string]any{
			"type":        "string",
			"maxLength":   200,
			"description": "Optional one-line note shown alongside the image (what it is, why you're showing it).",
		},
	},
}

// deps are the external effects one extension instance uses.
type deps struct {
	loader     loader
	opener     opener
	viewerRoot func() string
	now        func() time.Time
	getenv     func(string) string
}

type imgview struct {
	deps
	cacheMu sync.Mutex
	images  map[imageKey]*tui.Image
}

type imageKey struct {
	digest [sha256.Size]byte
	mime   string
}

// Extension constructs the imgview extension.
func Extension() *sdk.Extension {
	processes := &processOpener{command: "xdg-open"}
	ext := newExtension(deps{
		loader:     loader{client: &http.Client{}, home: os.UserHomeDir},
		opener:     processes,
		viewerRoot: viewerRoot,
		now:        time.Now,
		getenv:     os.Getenv,
	})
	ext.OnEvent(sdk.EventSessionShutdown, func(sdk.Context, map[string]any) (any, error) {
		processes.Wait(2 * time.Second)
		return nil, nil
	})
	return ext
}

func newExtension(d deps) *sdk.Extension {
	v := &imgview{deps: d, images: map[imageKey]*tui.Image{}}
	ext := sdk.New("imgview")
	ext.MessageRenderer(messageType, v.renderMessage)
	ext.RegisterTool(sdk.ToolDefinition{
		Name:             "show_image",
		Label:            "Show image",
		Description:      toolDescription,
		PromptSnippet:    "show_image — display an image to the user inline in the terminal and/or in the browser",
		PromptGuidelines: toolGuidelines,
		Parameters:       toolParameters,
		Execute:          v.showImage,
	})
	ext.Command("imgcat", "Render an image inline in the terminal: /imgcat <path|url|data:uri>", func(ctx sdk.Context, args string) error {
		return v.runCommand(ctx, args, modeTerminal)
	})
	ext.Command("imgshow", "Open an image in the default browser: /imgshow <path|url|data:uri>", func(ctx sdk.Context, args string) error {
		return v.runCommand(ctx, args, modeBrowser)
	})
	ext.Command("imgboth", "Render an image inline AND open it in the browser: /imgboth <path|url|data:uri>", func(ctx sdk.Context, args string) error {
		return v.runCommand(ctx, args, modeBoth)
	})
	return ext
}

// launch writes a viewer and starts the opener. path is non-empty only when
// the opener actually started.
func (v *imgview) launch(img *resolvedImage) (path, command string, err error) {
	viewer, err := writeViewer(img, v.viewerRoot(), v.now())
	if err != nil {
		return "", "", err
	}
	name, args, err := v.opener.Open(viewer)
	if err != nil {
		return "", "", err
	}
	return viewer, strings.Join(append([]string{name}, args...), " "), nil
}

// fitsFrame reports whether an image can travel in one extension wire frame.
func fitsFrame(img *resolvedImage) bool {
	return base64.StdEncoding.EncodedLen(len(img.Bytes))+frameHeadroom <= sdk.MaxFrameSize
}

func transportLimitText(img *resolvedImage) string {
	return fmt.Sprintf("image is %s bytes (%s base64 bytes), above what one PiG extension message can carry (%s bytes)",
		groupThousands(len(img.Bytes)), groupThousands(base64.StdEncoding.EncodedLen(len(img.Bytes))), groupThousands(sdk.MaxFrameSize-frameHeadroom))
}

func (v *imgview) showImage(ctx sdk.Context, args map[string]any) (any, error) {
	source, _ := args["source"].(string)
	caption, _ := args["caption"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = modeTerminal
	}
	if mode != modeTerminal && mode != modeBrowser && mode != modeBoth {
		return nil, fmt.Errorf("show_image failed: unsupported mode %q", mode)
	}
	_ = ctx.OnUpdate(sdk.ToolResult{Content: "Loading " + source + "...", Details: map[string]any{"source": source, "mode": mode}})

	request, done := requestContext(ctx)
	defer done()
	img, err := v.loader.resolve(request, source, ctx.Cwd())
	if err != nil {
		return nil, fmt.Errorf("show_image failed: %s", errorText(err))
	}
	if !isSupportedImageMime(img.MimeType) {
		return nil, fmt.Errorf("Refusing to show %s: detected MIME %s is not a supported image type.", img.SourceLabel, img.MimeType)
	}
	inline := mode == modeTerminal || mode == modeBoth
	sendable := fitsFrame(img)
	if mode == modeTerminal && !sendable {
		return nil, fmt.Errorf("show_image failed: %s; retry with mode='browser'", transportLimitText(img))
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("show_image failed: %v", err)
	}

	size := len(img.Bytes)
	var browserPath, browserError, openCommand string
	if mode == modeBrowser || mode == modeBoth {
		browserPath, openCommand, err = v.launch(img)
		if err != nil {
			browserError = err.Error()
		}
	}

	lines := []string{fmt.Sprintf("Showed %s (%s, %s bytes) via mode=%s.", img.SourceLabel, img.MimeType, groupThousands(size), mode)}
	if caption != "" {
		lines = append(lines, "Caption: "+caption)
	}
	if browserPath != "" {
		lines = append(lines, "Browser: opened "+browserPath+".")
	}
	if browserError != "" {
		lines = append(lines, "Browser open failed: "+browserError+".")
	}
	if size > softMaxBytes {
		lines = append(lines, fmt.Sprintf("Note: image is %s bytes (> %s soft cap); inline rendering still works but the encoded form is large in context.", groupThousands(size), groupThousands(softMaxBytes)))
	}
	if inline && !sendable {
		lines = append(lines, "Note: inline image omitted: "+transportLimitText(img)+".")
	}
	if inline && sendable && !v.terminalLikelySupportsImages() {
		lines = append(lines, "Note: this terminal doesn't appear to advertise inline image support (TERM_PROGRAM/KITTY_WINDOW_ID/WEZTERM not set). PiG will still attempt to render; if you see only a placeholder, retry with mode='browser'.")
	}

	details := map[string]any{"source": source, "resolved": img.SourceLabel, "mimeType": img.MimeType, "bytes": size, "mode": mode}
	for key, value := range map[string]string{"browserPath": browserPath, "browserError": browserError, "openCommand": openCommand, "caption": caption} {
		if value != "" {
			details[key] = value
		}
	}
	result := sdk.ToolResult{Content: strings.Join(lines, "\n"), Details: details, IsError: mode == modeBrowser && browserError != ""}
	if inline && sendable {
		result.Images = []sdk.ImageContent{{Data: base64.StdEncoding.EncodeToString(img.Bytes), MimeType: img.MimeType}}
	}
	return result, nil
}

// requestContext adapts the SDK request lifetime for net/http and joins its
// watcher before returning.
func requestContext(ctx sdk.Context) (context.Context, func()) {
	request, cancel := context.WithCancel(context.Background())
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			cancel()
		case <-request.Done():
		}
	}()
	return request, func() { cancel(); <-joined }
}

func errorText(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return err.Error()
}

func commandName(mode string) string {
	switch mode {
	case modeTerminal:
		return "imgcat"
	case modeBrowser:
		return "imgshow"
	}
	return "imgboth"
}

func (v *imgview) runCommand(ctx sdk.Context, args, mode string) error {
	source := jsTrim(args)
	if source == "" {
		ctx.Notify("Usage: /"+commandName(mode)+" <path|url|data:uri>", "warning")
		return nil
	}
	request, done := requestContext(ctx)
	defer done()
	img, err := v.loader.resolve(request, source, ctx.Cwd())
	if err != nil {
		ctx.Notify("imgview: "+errorText(err), "error")
		return nil
	}
	if !isSupportedImageMime(img.MimeType) {
		ctx.Notify(fmt.Sprintf("imgview: %s has unsupported MIME %s.", img.SourceLabel, img.MimeType), "error")
		return nil
	}
	inline := mode == modeTerminal || mode == modeBoth
	if mode == modeTerminal && !fitsFrame(img) {
		ctx.Notify("imgview: "+transportLimitText(img)+"; use /imgshow instead.", "error")
		return nil
	}

	var browserPath string
	if mode == modeBrowser || mode == modeBoth {
		browserPath, _, err = v.launch(img)
		if err != nil {
			ctx.Notify("imgview: failed to open browser: "+err.Error(), "error")
			if mode == modeBrowser {
				return nil
			}
		}
	}

	if inline {
		if !fitsFrame(img) {
			ctx.Notify("imgview: inline image omitted: "+transportLimitText(img)+".", "error")
		} else {
			err := ctx.SendCustomMessage(sdk.CustomMessage{
				CustomType: messageType,
				Content:    fmt.Sprintf("imgview: %s (%s, %s bytes)", img.SourceLabel, img.MimeType, groupThousands(len(img.Bytes))),
				Display:    true,
				Details: map[string]any{
					"source": source, "resolved": img.SourceLabel, "mimeType": img.MimeType, "bytes": len(img.Bytes),
					"image": map[string]any{"data": base64.StdEncoding.EncodeToString(img.Bytes), "mimeType": img.MimeType},
				},
			}, sdk.SendMessageOptions{DeliverAs: "followUp", TriggerTurn: sdk.Bool(false)})
			if err != nil {
				ctx.Notify("imgview: failed to add image message: "+err.Error(), "error")
				return nil
			}
		}
	}

	summary := []string{"imgview: " + img.SourceLabel, fmt.Sprintf("mime=%s bytes=%s mode=%s", img.MimeType, groupThousands(len(img.Bytes)), mode)}
	if browserPath != "" {
		summary = append(summary, "browser="+browserPath)
	}
	ctx.Notify(strings.Join(summary, "\n"), "info")
	return nil
}

// renderMessage draws the slash-command transcript entry: the label and the
// image through PiG's public TUI Image component (60-cell maximum, as upstream).
func (v *imgview) renderMessage(ctx sdk.Context, message map[string]any, _ sdk.MessageRenderOptions, width int) ([]string, error) {
	theme := ctx.UITheme()
	details, _ := message["details"].(map[string]any)
	label, ok := message["content"].(string)
	if !ok {
		resolved, _ := details["resolved"].(string)
		if resolved == "" {
			resolved = "<image>"
		}
		label = "imgview: " + resolved
	}
	width = max(1, width)
	lines := widthx.WrapTextWithAnsi(theme.Fg("accent", label), width)
	image, _ := details["image"].(map[string]any)
	data, _ := image["data"].(string)
	mimeType, _ := image["mimeType"].(string)
	if data == "" || mimeType == "" {
		return lines, nil
	}
	component := v.image(data, mimeType, theme)
	return append(append(lines, ""), component.Render(width)...), nil
}

// image returns a cached component so re-rendering one message keeps a stable
// Kitty image identity instead of transmitting a fresh image every frame.
func (v *imgview) image(data, mimeType string, theme sdk.UITheme) *tui.Image {
	key := imageKey{digest: sha256.Sum256([]byte(data)), mime: mimeType}
	v.cacheMu.Lock()
	defer v.cacheMu.Unlock()
	if image, ok := v.images[key]; ok {
		return image
	}
	if len(v.images) >= 64 {
		clear(v.images)
	}
	image := tui.NewImage(data, mimeType, tui.ImageOptions{MaxWidthCells: 60}, nil)
	image.Theme = tui.ImageTheme{FallbackColor: func(s string) string { return theme.Fg("muted", s) }}
	v.images[key] = image
	return image
}

// terminalLikelySupportsImages is upstream's model-facing hint only; PiG's
// renderer makes the actual decision.
func (v *imgview) terminalLikelySupportsImages() bool {
	if v.getenv("KITTY_WINDOW_ID") != "" || v.getenv("WEZTERM_EXECUTABLE") != "" || v.getenv("WEZTERM_PANE") != "" {
		return true
	}
	program := strings.ToLower(v.getenv("TERM_PROGRAM"))
	for _, name := range []string{"iterm", "wezterm", "ghostty", "vscode"} {
		if strings.Contains(program, name) {
			return true
		}
	}
	term := strings.ToLower(v.getenv("TERM"))
	return strings.Contains(term, "kitty") || strings.Contains(term, "wezterm")
}
