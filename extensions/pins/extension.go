// Copyright (c) 2026 s4lv0
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

// Package pins implements the /pin PiG extension: pin assistant messages and
// recall them in a dismissible overlay browser.
//
// Command and state semantics are adapted from s4lv0/pi-pins (commit
// 776217ccdce52aa0ae7794998847ff2253677250); the borderless browser follows
// kmet's pins presentation. Pins are branch-local pin-state session entries,
// read from the active branch on every command.
package pins

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

var helpText = strings.Join([]string{
	"/pin [label]    Pin the latest nonempty assistant text",
	"/pin pick       Pick one of the ten latest assistant texts",
	"/pin show [n]   Browse pins, optionally at #n",
	"/pin list [n]   Alias for show",
	"/pin rm <n>     Remove pin #n",
	"/pin clear      Remove all pins (IDs restart at 1)",
	"/pin help       Show these instructions",
	"",
	"Viewer: " + browserKeys,
	"Pins follow the session branch and survive /reload, restart, and /resume.",
}, "\n")

var subcommands = []sdk.AutocompleteItem{
	{Value: "pick", Label: "pick", Description: "Pick a recent assistant message"},
	{Value: "show", Label: "show", Description: "Browse pins, optionally at #n"},
	{Value: "list", Label: "list", Description: "Alias for show"},
	{Value: "rm", Label: "rm", Description: "Remove pin #n"},
	{Value: "clear", Label: "clear", Description: "Remove all pins; restart IDs"},
	{Value: "help", Label: "help", Description: "Show instructions"},
}

type pins struct {
	now func() time.Time
}

// Extension constructs the pins extension.
func Extension() *sdk.Extension {
	return newExtension(time.Now)
}

func newExtension(now func() time.Time) *sdk.Extension {
	p := &pins{now: now}
	ext := sdk.New("pins")
	ext.RegisterCommand("pin", sdk.CommandOptions{
		Description: "Pin assistant messages and recall them in a full-width viewer (/pin help)",
		GetArgumentCompletions: func(prefix string) ([]sdk.AutocompleteItem, error) {
			var items []sdk.AutocompleteItem
			for _, item := range subcommands {
				if strings.HasPrefix(item.Value, prefix) {
					items = append(items, item)
				}
			}
			return items, nil
		},
		Handler: p.command,
	})
	return ext
}

func (p *pins) branch(ctx sdk.Context) ([]map[string]any, error) {
	entries, err := ctx.SessionManager().GetBranch(nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to read session: %v", err)
	}
	return entries, nil
}

func (p *pins) state(ctx sdk.Context) (State, []map[string]any, error) {
	branch, err := p.branch(ctx)
	if err != nil {
		return State{}, nil, err
	}
	state, err := restoreState(branch)
	return state, branch, err
}

func (p *pins) persist(ctx sdk.Context, state State) error {
	if err := ctx.AppendEntry(stateType, state); err != nil {
		return fmt.Errorf("pin state was not saved: %v", err)
	}
	return nil
}

func (p *pins) command(ctx sdk.Context, args string) error {
	if err := p.run(ctx, parseCommand(args)); err != nil {
		ctx.Notify("pins: "+err.Error(), "error")
	}
	return nil
}

func (p *pins) run(ctx sdk.Context, cmd command) error {
	switch cmd.action {
	case "help":
		ctx.Notify(helpText, "info")
		return nil
	case "clear":
		if err := p.persist(ctx, emptyState()); err != nil {
			return err
		}
		ctx.Notify("All pins removed", "info")
		return nil
	case "rm":
		state, _, err := p.state(ctx)
		if err != nil {
			return err
		}
		id, ok := parseID(cmd.arg)
		var pin *Pin
		for i := range state.Pins {
			if ok && state.Pins[i].ID == id {
				pin = &state.Pins[i]
			}
		}
		if pin == nil {
			if cmd.arg == "" {
				ctx.Notify("Usage: /pin rm <n>", "error")
			} else {
				ctx.Notify("No pin #"+cmd.arg, "error")
			}
			return nil
		}
		label := pin.Label
		if err := p.persist(ctx, removePin(state, id)); err != nil {
			return err
		}
		ctx.Notify(fmt.Sprintf("Removed pin #%d %q", id, label), "info")
		return nil
	case "show":
		state, _, err := p.state(ctx)
		if err != nil {
			return err
		}
		index := 0
		if cmd.arg != "" {
			index = -1
			if id, ok := parseID(cmd.arg); ok {
				for i, pin := range state.Pins {
					if pin.ID == id {
						index = i
					}
				}
			}
			if index < 0 {
				ctx.Notify("No pin #"+cmd.arg, "error")
				return nil
			}
		}
		if len(state.Pins) == 0 {
			ctx.Notify("No pins yet — use /pin first", "warning")
			return nil
		}
		return p.browse(ctx, state.Pins, index)
	case "pick":
		state, branch, err := p.state(ctx)
		if err != nil {
			return err
		}
		candidates := recentAssistants(branch)
		if len(candidates) == 0 {
			ctx.Notify("No assistant messages to pin", "warning")
			return nil
		}
		labels := make([]string, len(candidates))
		for i, c := range candidates {
			// Labels are unique: each candidate has a distinct "ago" count.
			labels[i] = c.label()
		}
		choice, ok, err := ctx.Select("Pin which message?", labels)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		for i, label := range labels {
			if label == choice {
				return p.add(ctx, state, candidates[i].Text, "")
			}
		}
		return nil
	default:
		state, branch, err := p.state(ctx)
		if err != nil {
			return err
		}
		candidates := recentAssistants(branch)
		if len(candidates) == 0 {
			ctx.Notify("No assistant message to pin", "warning")
			return nil
		}
		return p.add(ctx, state, candidates[0].Text, cmd.label)
	}
}

func (p *pins) add(ctx sdk.Context, state State, text, label string) error {
	next, pin := addPin(state, text, label, p.now().UnixMilli())
	if err := p.persist(ctx, next); err != nil {
		return err
	}
	ctx.Notify("📌 Pinned as #"+strconv.Itoa(pin.ID)+" \""+pin.Label+"\" — recall with /pin show "+strconv.Itoa(pin.ID), "info")
	return nil
}

// browse opens the overlay browser on an immutable snapshot. It needs PiG's
// interactive terminal ("tui" mode): RPC can report UI support without a
// terminal browser.
func (p *pins) browse(ctx sdk.Context, pins []Pin, index int) error {
	if ctx.Mode() != "tui" || !ctx.HasUI() {
		ctx.Notify("/pin viewer requires interactive TUI mode", "warning")
		return nil
	}
	view := newBrowser(pins, index, ctx.UITheme, ctx.Height, ctx.Width)
	_, err := ctx.Custom(view, overlayOptions())
	return err
}
