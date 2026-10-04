// Copyright (c) 2026 s4lv0
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

const (
	stateType = "pin-state"
	pickCount = 10
)

// Pin is one saved assistant text. The JSON shape matches upstream pi-pins.
type Pin struct {
	ID       int    `json:"id"`
	Label    string `json:"label"`
	Text     string `json:"text"`
	PinnedAt int64  `json:"pinnedAt"`
}

// State is one pin-state snapshot.
type State struct {
	Pins   []Pin `json:"pins"`
	NextID int   `json:"nextId"`
}

func emptyState() State { return State{Pins: []Pin{}, NextID: 1} }

var errInvalidState = errors.New("Invalid saved pin-state; no pins were changed")

// isJSSpace matches ECMAScript \s, including BOM but not NEL.
func isJSSpace(r rune) bool {
	return unicode.Is(unicode.Zs, r) || strings.ContainsRune("\t\n\v\f\r\u2028\u2029\uFEFF", r)
}

func jsTrim(s string) string { return strings.TrimFunc(s, isJSSpace) }

// jsSlice returns the first n UTF-16 code units of s without splitting a
// surrogate pair, mirroring String.prototype.slice lengths.
func jsSlice(s string, n int) (string, bool) {
	units := 0
	for i, r := range s {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > n {
			return s[:i], true
		}
		units += width
	}
	return s, false
}

func jsLength(s string) int { return len(utf16.Encode([]rune(s))) }

// extractText joins assistant text blocks; thinking, tool calls and images are
// excluded. String content is accepted as text.
func extractText(content any) string {
	switch value := content.(type) {
	case string:
		return jsTrim(value)
	case []any:
		parts := []string{}
		for _, block := range value {
			part, ok := block.(map[string]any)
			if !ok || part["type"] != "text" {
				continue
			}
			if text, ok := part["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return jsTrim(strings.Join(parts, "\n"))
	}
	return ""
}

// jsSpaceClass is the ECMAScript \s character class body for RE2.
const jsSpaceClass = `\t\n\x0B\f\r \x{00A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}`

var labelPrefix = regexp.MustCompile("^[#*\\-|>`" + jsSpaceClass + "]+")

func autoLabel(text string) string {
	first := ""
	for _, line := range strings.Split(text, "\n") {
		if jsTrim(line) != "" {
			first = line
			break
		}
	}
	cleaned := jsTrim(labelPrefix.ReplaceAllString(first, ""))
	if jsLength(cleaned) > 42 {
		head, _ := jsSlice(cleaned, 42)
		return head + "…"
	}
	if cleaned == "" {
		return "pin"
	}
	return cleaned
}

var whitespaceRun = regexp.MustCompile("[" + jsSpaceClass + "]+")

func preview(text string, maxUnits int) string {
	flat := jsTrim(whitespaceRun.ReplaceAllString(text, " "))
	if jsLength(flat) > maxUnits {
		head, _ := jsSlice(flat, maxUnits)
		return head + "…"
	}
	return flat
}

type candidate struct {
	Text string
	Ago  int
}

func (c candidate) label() string {
	when := "last"
	if c.Ago != 1 {
		when = strconv.Itoa(c.Ago) + " back"
	}
	return when + " · " + preview(c.Text, 80)
}

// recentAssistants returns the newest ten nonempty assistant texts on branch.
// Ago counts textless assistant messages too, as upstream.
func recentAssistants(branch []map[string]any) []candidate {
	out := []candidate{}
	ago := 0
	for i := len(branch) - 1; i >= 0 && len(out) < pickCount; i-- {
		entry := branch[i]
		if entry["type"] != "message" {
			continue
		}
		message, ok := entry["message"].(map[string]any)
		if !ok || message["role"] != "assistant" {
			continue
		}
		ago++
		if text := extractText(message["content"]); text != "" {
			out = append(out, candidate{Text: text, Ago: ago})
		}
	}
	return out
}

// restoreState returns the last pin-state snapshot on branch. A corrupt
// snapshot is an error, never silently replaced. A missing or stale nextId is
// repaired above the highest existing ID.
func restoreState(branch []map[string]any) (State, error) {
	var last map[string]any
	found := false
	for _, entry := range branch {
		if entry["type"] == "custom" && entry["customType"] == stateType {
			data, ok := entry["data"].(map[string]any)
			if !ok {
				return State{}, errInvalidState
			}
			last, found = data, true
		}
	}
	if !found {
		return emptyState(), nil
	}
	state := emptyState()
	if raw, present := last["pins"]; present && raw != nil {
		items, ok := raw.([]any)
		if !ok {
			return State{}, errInvalidState
		}
		seen := map[int]bool{}
		for _, item := range items {
			pin, ok := decodePin(item)
			if !ok || seen[pin.ID] {
				return State{}, errInvalidState
			}
			seen[pin.ID] = true
			state.Pins = append(state.Pins, pin)
		}
	}
	highest := 0
	for _, pin := range state.Pins {
		highest = max(highest, pin.ID)
	}
	next := 1
	if id, ok := positiveInt(last["nextId"]); ok {
		next = id
	}
	state.NextID = max(highest+1, next)
	return state, nil
}

func positiveInt(value any) (int, bool) {
	number, ok := value.(float64)
	if !ok || number < 1 || number > 1<<53 || number != math.Trunc(number) {
		return 0, false
	}
	return int(number), true
}

func decodePin(item any) (Pin, bool) {
	fields, ok := item.(map[string]any)
	if !ok {
		return Pin{}, false
	}
	id, ok := positiveInt(fields["id"])
	if !ok {
		return Pin{}, false
	}
	label, labelOK := fields["label"].(string)
	text, textOK := fields["text"].(string)
	at, atOK := fields["pinnedAt"].(float64)
	if !labelOK || !textOK || !atOK || at < 0 || at != math.Trunc(at) || at > 1<<53 {
		return Pin{}, false
	}
	return Pin{ID: id, Label: label, Text: text, PinnedAt: int64(at)}, true
}

func addPin(state State, text, label string, now int64) (State, Pin) {
	pin := Pin{ID: state.NextID, Label: jsTrim(label), Text: text, PinnedAt: now}
	if pin.Label == "" {
		pin.Label = autoLabel(text)
	}
	next := State{Pins: append(append([]Pin{}, state.Pins...), pin), NextID: state.NextID + 1}
	return next, pin
}

func removePin(state State, id int) State {
	kept := []Pin{}
	for _, pin := range state.Pins {
		if pin.ID != id {
			kept = append(kept, pin)
		}
	}
	return State{Pins: kept, NextID: state.NextID}
}

var digits = regexp.MustCompile(`^[0-9]{1,15}$`)

// parseID accepts a positive decimal pin number.
func parseID(s string) (int, bool) {
	if !digits.MatchString(s) {
		return 0, false
	}
	id, err := strconv.Atoi(s)
	return id, err == nil && id > 0
}

type command struct {
	action string // pin, pick, show, rm, clear, help
	arg    string
	label  string
}

// parseCommand follows upstream: a first word naming a subcommand counts only
// when no text follows it, or when it accepts an argument (show/rm, and list
// with a pin number).
// Otherwise the whole input is a free label. Subcommands are case-insensitive.
func parseCommand(args string) command {
	trimmed := jsTrim(args)
	fields := strings.FieldsFunc(trimmed, isJSSpace)
	if len(fields) == 0 {
		return command{action: "pin"}
	}
	sub := strings.ToLower(fields[0])
	rest := jsTrim(trimmed[len(fields[0]):])
	switch sub {
	case "pick", "clear", "help":
		if rest == "" {
			return command{action: sub}
		}
	case "show":
		return command{action: "show", arg: rest}
	case "list":
		// "list" is an alias for show; upstream documents "/pin list of
		// plugins" as a free label, so only an empty or numeric argument
		// selects the alias.
		if rest == "" || digits.MatchString(rest) {
			return command{action: "show", arg: rest}
		}
	case "rm":
		return command{action: "rm", arg: rest}
	}
	return command{action: "pin", label: trimmed}
}
