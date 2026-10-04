// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package pins

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func assistantEntry(content any) map[string]any {
	return map[string]any{"type": "message", "message": map[string]any{"role": "assistant", "content": content}}
}

func stateEntry(data any) map[string]any {
	return map[string]any{"type": "custom", "customType": stateType, "data": jsonRoundTrip(data)}
}

// jsonRoundTrip gives values the decoded wire shape (float64 numbers, []any).
func jsonRoundTrip(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(raw, &out)
	return out
}

func TestExtractText(t *testing.T) {
	if got := extractText("  hello  "); got != "hello" {
		t.Fatal(got)
	}
	blocks := jsonRoundTrip([]any{
		map[string]any{"type": "text", "text": " one"},
		map[string]any{"type": "thinking", "thinking": "secret"},
		map[string]any{"type": "image", "data": "image"},
		map[string]any{"type": "toolCall", "name": "bash"},
		nil, map[string]any{"type": "text", "text": 42},
		map[string]any{"type": "text", "text": "two "},
	})
	if got := extractText(blocks); got != "one\ntwo" {
		t.Fatalf("%q", got)
	}
	for _, content := range []any{nil, 3.0, map[string]any{}, []any{}, jsonRoundTrip([]any{map[string]any{"type": "thinking", "text": "hidden"}})} {
		if got := extractText(content); got != "" {
			t.Fatalf("%v -> %q", content, got)
		}
	}
}

func TestAutoLabelAndPreview(t *testing.T) {
	for in, want := range map[string]string{
		"\n  ## **Plan\nNext":   "Plan",
		"#* | ` > -":            "pin",
		strings.Repeat("a", 43): strings.Repeat("a", 42) + "…",
		strings.Repeat("a", 42): strings.Repeat("a", 42),
		"> quoted line":         "quoted line",
		"   \n\t\n":             "pin",
		// 41 ASCII + one astral character is 43 UTF-16 units; the pair is not split.
		strings.Repeat("b", 41) + "🐈x": strings.Repeat("b", 41) + "…",
	} {
		if got := autoLabel(in); got != want {
			t.Errorf("autoLabel(%q) = %q, want %q", in, got, want)
		}
	}
	for _, tc := range []struct {
		in   string
		max  int
		want string
	}{
		{"one\n two\tthree", 80, "one two three"},
		{"abcdefghi", 6, "abcdef…"},
		{"abc", 3, "abc"},
		{"a　\u2028b\vc", 80, "a b c"},
	} {
		if got := preview(tc.in, tc.max); got != tc.want {
			t.Errorf("preview(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRecentAssistantsCountsTextlessDistance(t *testing.T) {
	var branch []map[string]any
	for i := range 12 {
		branch = append(branch, assistantEntry(jsonRoundTrip([]any{map[string]any{"type": "text", "text": strconv.Itoa(i)}})))
	}
	branch = append(branch,
		map[string]any{"type": "message", "message": map[string]any{"role": "user", "content": "not assistant"}},
		map[string]any{"type": "custom", "customType": stateType, "data": map[string]any{}},
		assistantEntry(jsonRoundTrip([]any{map[string]any{"type": "thinking", "thinking": "hidden"}})))
	got := recentAssistants(branch)
	if len(got) != 10 || got[0] != (candidate{"11", 2}) || got[9] != (candidate{"2", 11}) {
		t.Fatalf("%+v", got)
	}
	if got[0].label() != "2 back · 11" || (candidate{"x", 1}).label() != "last · x" {
		t.Fatal(got[0].label())
	}
	if len(recentAssistants(nil)) != 0 {
		t.Fatal("empty branch")
	}
}

func TestStateLifecycleAndRepair(t *testing.T) {
	one, pin := addPin(emptyState(), "text", "", 123)
	if pin != (Pin{1, "text", "text", 123}) || one.NextID != 2 {
		t.Fatalf("%+v %+v", one, pin)
	}
	two, pin := addPin(one, "text", " my label ", 124)
	if pin.Label != "my label" || len(one.Pins) != 1 {
		t.Fatal("addPin must not mutate its input", pin, one)
	}
	restored, err := restoreState([]map[string]any{stateEntry(one), assistantEntry("x"), stateEntry(two)})
	if err != nil || !reflect.DeepEqual(restored, two) {
		t.Fatalf("last snapshot wins: %+v %v", restored, err)
	}
	removed := removePin(two, 1)
	if len(removed.Pins) != 1 || removed.Pins[0].ID != 2 || removed.NextID != 3 {
		t.Fatalf("%+v", removed)
	}
	for _, data := range []map[string]any{
		{"pins": removed.Pins},
		{"pins": removed.Pins, "nextId": 1},
		{"pins": removed.Pins, "nextId": "9"},
		{"pins": removed.Pins, "nextId": 1.5},
	} {
		got, err := restoreState([]map[string]any{stateEntry(data)})
		if err != nil || got.NextID != 3 {
			t.Fatalf("repair %v: %+v %v", data, got, err)
		}
	}
	got, _ := restoreState([]map[string]any{stateEntry(map[string]any{"pins": removed.Pins, "nextId": 9})})
	if got.NextID != 9 {
		t.Fatal("a higher nextId is kept, so removed IDs are not reused")
	}
	for _, branch := range [][]map[string]any{nil, {stateEntry(emptyState())}, {stateEntry(map[string]any{})}, {stateEntry(map[string]any{"pins": nil})}} {
		if got, err := restoreState(branch); err != nil || !reflect.DeepEqual(got, emptyState()) {
			t.Fatalf("%v: %+v %v", branch, got, err)
		}
	}
}

func TestCorruptStateIsNotDiscarded(t *testing.T) {
	pin := map[string]any{"id": 1, "label": "x", "text": "x", "pinnedAt": 1}
	without := func(key string) map[string]any {
		out := map[string]any{}
		for k, v := range pin {
			if k != key {
				out[k] = v
			}
		}
		return out
	}
	with := func(key string, value any) map[string]any {
		out := without(key)
		out[key] = value
		return out
	}
	for _, data := range []any{nil, "broken", map[string]any{"pins": "broken"}, map[string]any{"pins": []any{pin, pin}},
		map[string]any{"pins": []any{without("text")}}, map[string]any{"pins": []any{with("id", 0)}},
		map[string]any{"pins": []any{with("id", 1.5)}}, map[string]any{"pins": []any{with("pinnedAt", -1)}},
		map[string]any{"pins": []any{with("label", 3)}}, map[string]any{"pins": []any{"x"}}} {
		// A corrupt snapshot after a valid one still fails: the last one wins.
		branch := []map[string]any{stateEntry(map[string]any{"pins": []any{pin}}), stateEntry(data)}
		if _, err := restoreState(branch); !errors.Is(err, errInvalidState) {
			t.Fatalf("%v: %v", data, err)
		}
	}
}

func TestParseCommand(t *testing.T) {
	for args, want := range map[string]command{
		"":                 {action: "pin"},
		"  my   label  ":   {action: "pin", label: "my   label"},
		"PiCk":             {action: "pick"},
		"pick some answer": {action: "pin", label: "pick some answer"},
		"help topics":      {action: "pin", label: "help topics"},
		"clear plans":      {action: "pin", label: "clear plans"},
		" show  02 ":       {action: "show", arg: "02"},
		"SHOW":             {action: "show"},
		"list":             {action: "show"},
		"list 3":           {action: "show", arg: "3"},
		"list of plugins":  {action: "pin", label: "list of plugins"},
		"show of plugins":  {action: "show", arg: "of plugins"},
		"rm":               {action: "rm"},
		"rm 2":             {action: "rm", arg: "2"},
		"HELP":             {action: "help"},
		"clear":            {action: "clear"},
		"　pick　":           {action: "pick"},
	} {
		if got := parseCommand(args); got != want {
			t.Errorf("parseCommand(%q) = %+v, want %+v", args, got, want)
		}
	}
	for _, id := range []string{"0", "-1", "1.5", "1e2", "abc", "", strings.Repeat("9", 30), " 1"} {
		if _, ok := parseID(id); ok {
			t.Errorf("parseID(%q) accepted", id)
		}
	}
	if id, ok := parseID("01"); !ok || id != 1 {
		t.Fatal("01")
	}
}
