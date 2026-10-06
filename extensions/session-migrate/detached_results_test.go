// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT
package sessionmigrate

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func detachedResult(id, call string) object {
	r := msg(id, "a", "user", []any{object{"type": "tool_result", "tool_use_id": call, "content": "RECOVERED", "is_error": true}})
	r["sourceToolAssistantUUID"] = "a"
	return r
}
func detachedRows(results ...object) []object {
	rows := []object{msg("u", "", "user", "hello"), msg("a", "u", "assistant", []any{object{"type": "tool_use", "id": "c", "name": "Write", "input": object{}}})}
	rows = append(rows, results...)
	return append(rows, msg("next", "a", "assistant", "continued"), object{"type": "last-prompt", "leafUuid": "next"})
}

func TestRecoverDetachedResult(t *testing.T) {
	for _, before := range []bool{false, true} {
		rows := detachedRows(detachedResult("r", "c"))
		if before {
			rows[1], rows[2] = rows[2], rows[1]
		}
		s, err := ReadClaude(context.Background(), fixture(t, rows))
		if err != nil {
			t.Fatal(err)
		}
		if s.Report.SelectedRecords != 4 || s.Report.Preserved["tool_results"] != 1 || len(s.Entries) != 4 {
			t.Fatal(s.Report)
		}
		m := obj(s.Entries[2]["message"])
		if m["role"] != "toolResult" || m["toolCallId"] != "c" || m["isError"] != true {
			t.Fatal(m)
		}
		if obj(s.Entries[3]["message"])["role"] != "assistant" {
			t.Fatal("result not inserted before continuation")
		}
	}
}

func TestDetachedResultIsolation(t *testing.T) {
	cases := map[string]func(object){
		"sidechain":         func(r object) { r["isSidechain"] = true },
		"metadata":          func(r object) { r["isMeta"] = true },
		"summary":           func(r object) { r["isCompactSummary"] = true },
		"different-session": func(r object) { r["sessionId"] = "other" },
		"missing-session":   func(r object) { delete(r, "sessionId") },
		"missing-source":    func(r object) { delete(r, "sourceToolAssistantUUID") },
		"wrong-source":      func(r object) { r["sourceToolAssistantUUID"] = "u" },
		"wrong-parent":      func(r object) { r["parentUuid"] = "u" },
		"missing-uuid":      func(r object) { delete(r, "uuid") },
		"wrong-role":        func(r object) { obj(r["message"])["role"] = "assistant" },
		"sibling-text": func(r object) {
			obj(r["message"])["content"] = []any{object{"type": "tool_result", "tool_use_id": "c", "content": "ok"}, object{"type": "text", "text": "EXCLUDED"}}
		},
		"unrelated-call": func(r object) {
			obj(r["message"])["content"] = []any{object{"type": "tool_result", "tool_use_id": "other", "content": "ok"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := detachedResult("r", "c")
			mutate(r)
			_, err := ReadClaude(context.Background(), fixture(t, detachedRows(r)))
			if err == nil || !strings.Contains(err.Error(), "unresolved tool call") {
				t.Fatalf("want unresolved call, got %v", err)
			}
		})
	}
}

func TestDetachedResultAmbiguity(t *testing.T) {
	_, err := ReadClaude(context.Background(), fixture(t, detachedRows(detachedResult("r1", "c"), detachedResult("r2", "c"))))
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatal(err)
	}
}

func TestDoNotRecoverAlreadyResolvedOrInactiveCalls(t *testing.T) {
	rows := detachedRows(detachedResult("r", "c"), detachedResult("sibling", "c"))
	rows[len(rows)-2]["parentUuid"] = "r"
	rows = append(rows, msg("fork", "u", "assistant", []any{object{"type": "tool_use", "id": "inactive", "name": "Write", "input": object{}}}))
	r := detachedResult("inactive-result", "inactive")
	r["parentUuid"] = "fork"
	r["sourceToolAssistantUUID"] = "fork"
	rows = append(rows, r)
	s, err := ReadClaude(context.Background(), fixture(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	if s.Report.Preserved["tool_results"] != 1 || s.Report.Omitted["inactive_or_metadata_record"] != 3 {
		t.Fatal(s.Report)
	}
	data, _ := json.Marshal(s.Entries)
	if strings.Contains(string(data), "inactive") {
		t.Fatal("inactive branch leaked")
	}
}

func TestRecoverMultipleResults(t *testing.T) {
	rows := detachedRows(detachedResult("r1", "c"), detachedResult("r2", "d"))
	obj(rows[1]["message"])["content"] = append(obj(rows[1]["message"])["content"].([]any), object{"type": "tool_use", "id": "d", "name": "Read", "input": object{}})
	s, err := ReadClaude(context.Background(), fixture(t, rows))
	if err != nil {
		t.Fatal(err)
	}
	if s.Report.Preserved["tool_results"] != 2 || obj(s.Entries[2]["message"])["toolCallId"] != "c" || obj(s.Entries[3]["message"])["toolCallId"] != "d" {
		t.Fatal(s.Report)
	}
}
