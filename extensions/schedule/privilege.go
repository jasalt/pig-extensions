// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"strings"
	"sync"
)

// Tool policy. PiG's powershell tool is an execution surface like bash.
var (
	mutateTools  = setOf("edit", "write", "bash", "powershell")
	suggestBlock = setOf("bash", "powershell", "terminal_exec", "terminal_write", "terminal_write_file", "terminal_run", "terminal_start", "terminal_tools")
	peerTools    = setOf("agent_send", "agent_request")
	// readOnlyAllow is the strict read_only allowlist; unknown tools fail closed.
	// The mcp gateway is deliberately excluded.
	readOnlyAllow = setOf("read", "grep", "glob", "find", "ls", "list", "web_search", "web_read",
		"auggie_codebase-retrieval", "codebase-retrieval", "list_peers", "show_file", "show_image",
		"terminal_read", "terminal_list", "terminal_wait")
	scheduleMutations = setOf("create", "cancel", "enable", "disable", "run_now", "trust")
)

func setOf(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}

// maxReservations bounds scheduled prompts awaiting or owning a turn.
const maxReservations = 16

// guard enforces tier policy on scheduled turns.
//
// Ownership is exact: a delivery reserves its unique prompt text with its
// tier before submission. A tool call belongs to the turn opened by the
// latest user message on the active branch, so it is restricted only when
// that message is a reserved scheduled prompt. Follow-up prompts queued in
// one agent run each keep their own tier, and an ordinary user prompt never
// inherits a scheduled restriction. Settlement drops reservations whose
// prompt has already started.
type guard struct {
	mu       sync.Mutex
	reserved []reservation
	legacy   func() bool
}

type reservation struct {
	prompt  string
	tier    string
	started bool
}

func (g *guard) reserve(prompt, tier string) func() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for len(g.reserved) >= maxReservations {
		g.reserved = g.reserved[1:]
	}
	g.reserved = append(g.reserved, reservation{prompt: prompt, tier: tier})
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		for i, r := range g.reserved {
			if r.prompt == prompt {
				g.reserved = append(g.reserved[:i], g.reserved[i+1:]...)
				return
			}
		}
	}
}

func (g *guard) empty() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.reserved) == 0
}

// tierFor returns the tier owning a turn whose latest user text is prompt.
func (g *guard) tierFor(prompt string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.reserved {
		if g.reserved[i].prompt == prompt {
			g.reserved[i].started = true
			return g.reserved[i].tier, true
		}
	}
	return "", false
}

// settled discards reservations that already owned a turn: those seen by a
// tool call, and those whose prompt reached the branch.
func (g *guard) settled(branch []map[string]any) {
	seen := map[string]bool{}
	for _, text := range userTexts(branch) {
		seen[text] = true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	kept := g.reserved[:0]
	for _, r := range g.reserved {
		if !r.started && !seen[r.prompt] {
			kept = append(kept, r)
		}
	}
	g.reserved = kept
}

func (g *guard) clear() {
	g.mu.Lock()
	g.reserved = nil
	g.mu.Unlock()
}

type blockResult struct {
	block  bool
	reason string
}

// check applies tier policy to one tool call (input is the tool arguments).
func (g *guard) check(tier, toolName string, input map[string]any) blockResult {
	if tier == "" || tier == tierMutate {
		return blockResult{}
	}
	lower := strings.ToLower(toolName)
	action, _ := input["action"].(string)
	scheduleMutation := lower == "schedule" && scheduleMutations[action]
	switch tier {
	case tierReadOnly:
		switch {
		case scheduleMutation:
			return blockResult{true, labelPrefix + " blocked schedule " + action + ": active scheduled job is tier=read_only (schedule mutations need tier=mutate; list/history are allowed)"}
		case mutateTools[lower]:
			return blockResult{true, labelPrefix + " blocked " + toolName + ": active scheduled job is tier=read_only"}
		case peerTools[lower]:
			return blockResult{true, labelPrefix + " blocked " + toolName + ": active scheduled job is tier=read_only (peer messaging can drive other agents)"}
		case !g.legacy() && lower != "schedule" && !readOnlyAllow[lower]:
			return blockResult{true, labelPrefix + " blocked " + toolName + ": active scheduled job is tier=read_only, and " + toolName +
				" is not on the read-only allowlist (unknown tools fail closed). Set PIG_SCHEDULE_PRIVILEGE_MODE=legacy to relax to the core-tool blocklist."}
		}
	case tierSuggest:
		switch {
		case suggestBlock[lower]:
			return blockResult{true, labelPrefix + " blocked " + toolName + ": active scheduled job is tier=suggest (no shell/exec)"}
		case peerTools[lower]:
			return blockResult{true, labelPrefix + " blocked " + toolName + ": active scheduled job is tier=suggest (peer messaging can drive other agents)"}
		case scheduleMutation:
			return blockResult{true, labelPrefix + " blocked schedule " + action + ": active scheduled job is tier=suggest (schedule mutations need tier=mutate)"}
		}
	}
	return blockResult{}
}

// userTexts returns the text of every user message on branch, oldest first.
func userTexts(branch []map[string]any) []string {
	var out []string
	for _, entry := range branch {
		if text, ok := userText(entry); ok {
			out = append(out, text)
		}
	}
	return out
}

// latestUserText is the text of the newest user message on branch.
func latestUserText(branch []map[string]any) (string, bool) {
	for i := len(branch) - 1; i >= 0; i-- {
		if text, ok := userText(branch[i]); ok {
			return text, true
		}
	}
	return "", false
}

func userText(entry map[string]any) (string, bool) {
	if entry["type"] != "message" {
		return "", false
	}
	message, ok := entry["message"].(map[string]any)
	if !ok || message["role"] != "user" {
		return "", false
	}
	switch content := message["content"].(type) {
	case string:
		return content, true
	case []any:
		var parts []string
		for _, block := range content {
			if b, ok := block.(map[string]any); ok && b["type"] == "text" {
				if text, ok := b["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n"), true
	}
	return "", true
}
