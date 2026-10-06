// Copyright (c) 2026 Jarkko Saltiola
// Copyright (c) 2026 xhluca
// SPDX-License-Identifier: MIT
// Graph selection and portable-block behavior adapted from xhluca/session-migrate.
package sessionmigrate

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
	"unicode/utf8"
)

const maxBytes = 256 << 20
const maxLine = 32 << 20

type object = map[string]any

// Report contains counts and source identity, never conversation bodies.
type Report struct {
	SourceFormat    string         `json:"sourceFormat"`
	SourceSHA256    string         `json:"sourceSHA256"`
	Records         int            `json:"records"`
	SelectedRecords int            `json:"selectedRecords"`
	Preserved       map[string]int `json:"preserved"`
	Omitted         map[string]int `json:"omitted"`
}

// Transcript is the validated, active-branch projection. Entries are native
// PiG drafts; WriteSession assigns fresh identities and tree links.
type Transcript struct {
	Report  Report
	Title   string
	Entries []object
}

func str(v any) string { s, _ := v.(string); return s }
func obj(v any) object { m, _ := v.(map[string]any); return m }

// ReadClaude reads one regular file, bounded by 256 MiB and 32 MiB per record.
// It never prints source content or accesses credentials/network services.
func ReadClaude(ctx context.Context, path string) (*Transcript, error) {
	// Linux target: nonblocking open lets us reject FIFOs before a writer exists.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > maxBytes {
		return nil, fmt.Errorf("source must be a regular file of at most 256 MiB")
	}
	h := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(io.LimitReader(f, maxBytes+1), h))
	scanner.Buffer(make([]byte, 64<<10), maxLine)
	var records []object
	line, total := 0, 0
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line++
		raw := scanner.Bytes()
		total += len(raw) + 1
		if total > maxBytes {
			return nil, fmt.Errorf("source exceeds 256 MiB limit")
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		if !utf8.Valid(raw) {
			return nil, fmt.Errorf("invalid UTF-8 at line %d", line)
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var row object
		if err := decoder.Decode(&row); err != nil || row == nil {
			return nil, fmt.Errorf("invalid JSON object at line %d", line)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, fmt.Errorf("trailing JSON at line %d", line)
		}
		records = append(records, row)
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("source read failed (record limit 32 MiB)")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selected, err := activeBranch(records)
	if err != nil {
		return nil, err
	}
	selected, err = recoverToolResults(records, selected)
	if err != nil {
		return nil, err
	}
	s := &Transcript{Report: Report{SourceFormat: "claude", SourceSHA256: hex.EncodeToString(h.Sum(nil)), Records: len(records), SelectedRecords: len(selected), Preserved: map[string]int{}, Omitted: map[string]int{}}}
	selectedSet := map[objectKey]bool{}
	for _, i := range selected {
		selectedSet[objectKey(i)] = true
	}
	var customTitle, aiTitle string
	for i, row := range records {
		switch str(row["type"]) {
		case "custom-title":
			if title := str(row["customTitle"]); title != "" {
				customTitle = title
			}
		case "ai-title":
			if title := str(row["aiTitle"]); title != "" {
				aiTitle = title
			}
		case "last-prompt", "queue-operation":
		default:
			if !selectedSet[objectKey(i)] {
				s.Report.Omitted["inactive_or_metadata_record"]++
			}
		}
	}
	s.Title = customTitle
	if s.Title == "" {
		s.Title = aiTitle
	}
	calls, results := map[string]string{}, map[string]bool{}
	sessionIDs := map[string]bool{}
	summaries := map[string]bool{}
	for _, i := range selected {
		row := records[i]
		if row["isSidechain"] == true {
			return nil, fmt.Errorf("Claude active graph includes a sidechain; use the parent session")
		}
		if row["isCompactSummary"] == true {
			summaries[str(row["parentUuid"])] = true
		}
		if id := str(row["sessionId"]); id != "" {
			sessionIDs[id] = true
		}
	}
	if len(sessionIDs) > 1 {
		return nil, fmt.Errorf("Claude active graph contains mixed session IDs")
	}
	fallback := time.Now().UTC()
	for _, i := range selected {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row := records[i]
		timestamp := fallback
		if stamp := str(row["timestamp"]); stamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
				timestamp = t
			} else {
				s.Report.Omitted["invalid_timestamp"]++
			}
		}
		entryTime := timestamp.UTC().Format(time.RFC3339Nano)
		appendMessage := func(m object) {
			m["timestamp"] = timestamp.UnixMilli()
			s.Entries = append(s.Entries, object{"type": "message", "timestamp": entryTime, "message": m})
			s.Report.Preserved["messages"]++
		}
		if row["type"] == "system" && row["subtype"] == "compact_boundary" {
			if !summaries[str(row["uuid"])] {
				s.Report.Omitted["compaction_without_summary"]++
			}
			continue
		}
		if row["type"] != "user" && row["type"] != "assistant" || row["isMeta"] == true || row["isSidechain"] == true {
			s.Report.Omitted["active_metadata_record"]++
			continue
		}
		message := obj(row["message"])
		role := str(message["role"])
		if role == "" {
			role = str(row["type"])
		}
		if row["isCompactSummary"] == true {
			text := contentText(message["content"])
			if text == "" {
				return nil, fmt.Errorf("Claude compact summary has no portable text")
			}
			s.Entries = append(s.Entries, object{"type": "compaction", "timestamp": entryTime, "summary": text, "tokensBefore": 0})
			s.Report.Preserved["compactions"]++
			s.Report.Omitted["compaction_metadata"]++
			continue
		}
		if role != "user" && role != "assistant" {
			s.Report.Omitted["privileged_or_unknown_role"]++
			continue
		}
		blocks := []object{}
		flush := func() {
			if len(blocks) == 0 {
				return
			}
			m := object{"role": role, "content": blocks}
			if role == "assistant" {
				m["provider"] = "anthropic"
				m["api"] = "anthropic-messages"
				m["model"] = str(message["model"])
				if m["model"] == "" {
					m["model"] = "unknown"
				}
				m["usage"] = emptyUsage()
				m["stopReason"] = "stop"
				for _, b := range blocks {
					if b["type"] == "toolCall" {
						m["stopReason"] = "toolUse"
					}
				}
			}
			appendMessage(m)
			blocks = nil
		}
		var content []any
		switch value := message["content"].(type) {
		case string:
			content = []any{object{"type": "text", "text": value}}
		case []any:
			content = value
		default:
			return nil, fmt.Errorf("unsupported Claude content in selected record %d", i+1)
		}
		for _, value := range content {
			b := obj(value)
			switch str(b["type"]) {
			case "text":
				if text, ok := b["text"].(string); ok {
					blocks = append(blocks, object{"type": "text", "text": text})
					s.Report.Preserved["text_blocks"]++
				} else {
					return nil, fmt.Errorf("malformed text block in selected record %d", i+1)
				}
			case "thinking", "redacted_thinking":
				s.Report.Omitted["private_thinking"]++
			case "tool_use":
				id, name := str(b["id"]), str(b["name"])
				if role != "assistant" || id == "" || name == "" || obj(b["input"]) == nil || calls[id] != "" {
					return nil, fmt.Errorf("invalid or duplicate Claude tool call in selected record %d", i+1)
				}
				calls[id] = name
				blocks = append(blocks, object{"type": "toolCall", "id": id, "name": name, "arguments": b["input"]})
				s.Report.Preserved["tool_calls"]++
			case "tool_result":
				flush()
				id := str(b["tool_use_id"])
				if role != "user" || id == "" || calls[id] == "" || results[id] {
					return nil, fmt.Errorf("orphan or duplicate Claude tool result in selected record %d", i+1)
				}
				results[id] = true
				resultBlocks := portableResult(b["content"], &s.Report)
				appendMessage(object{"role": "toolResult", "toolCallId": id, "toolName": calls[id], "content": resultBlocks, "isError": b["is_error"] == true})
				s.Report.Preserved["tool_results"]++
			case "image":
				if image := portableImage(b); image != nil && role == "user" {
					blocks = append(blocks, image)
					s.Report.Preserved["images"]++
				} else {
					s.Report.Omitted["unsupported_image"]++
				}
			case "document":
				s.Report.Omitted["document"]++
			default:
				s.Report.Omitted["unknown_content_block"]++
			}
		}
		flush()
		if row["toolUseResult"] != nil || row["sourceToolAssistantUUID"] != nil {
			s.Report.Omitted["tool_result_metadata"]++
		}
	}
	if len(s.Entries) == 0 {
		return nil, fmt.Errorf("source has no resumable conversation context")
	}
	for id := range calls {
		if !results[id] {
			return nil, fmt.Errorf("source has an unresolved tool call; import a completed transcript")
		}
	}
	return s, nil
}

// recoverToolResults includes result-only children skipped by the active ancestry.
// Claude can continue from an assistant record rather than its result child. Only
// explicit, same-session links to selected calls qualify; never flatten a fork,
// import sibling conversation text, or synthesize a result.
func recoverToolResults(records []object, selected []int) ([]int, error) {
	selectedSet := map[int]bool{}
	owners := map[string]int{}
	resolved := map[string]bool{}
	for _, i := range selected {
		selectedSet[i] = true
		row := records[i]
		if row["isMeta"] == true || row["isSidechain"] == true {
			continue
		}
		blocks, _ := obj(row["message"])["content"].([]any)
		for _, value := range blocks {
			b := obj(value)
			if row["type"] == "assistant" && b["type"] == "tool_use" {
				owners[str(b["id"])] = i
			}
			if row["type"] == "user" && b["type"] == "tool_result" {
				resolved[str(b["tool_use_id"])] = true
			}
		}
	}
	candidates := map[string][]int{}
	for i, row := range records {
		if selectedSet[i] || row["type"] != "user" || row["isMeta"] == true || row["isSidechain"] == true || row["isCompactSummary"] == true || str(row["uuid"]) == "" {
			continue
		}
		message := obj(row["message"])
		if role := str(message["role"]); role != "" && role != "user" {
			continue
		}
		blocks, ok := message["content"].([]any)
		if !ok || len(blocks) == 0 {
			continue
		}
		eligible := true
		for _, value := range blocks {
			b := obj(value)
			id := str(b["tool_use_id"])
			owner, exists := owners[id]
			if b["type"] != "tool_result" || id == "" || !exists || resolved[id] {
				eligible = false
				break
			}
			call := records[owner]
			if str(call["sessionId"]) == "" || row["sessionId"] != call["sessionId"] || row["parentUuid"] != call["uuid"] || row["sourceToolAssistantUUID"] != call["uuid"] {
				eligible = false
				break
			}
		}
		if eligible {
			for _, value := range blocks {
				id := str(obj(value)["tool_use_id"])
				candidates[id] = append(candidates[id], i)
			}
		}
	}
	children := map[int][]int{}
	added := map[int]bool{}
	// Source order only orders explicitly linked siblings, not conversation forks.
	for i, row := range records {
		blocks, _ := obj(row["message"])["content"].([]any)
		for _, value := range blocks {
			id := str(obj(value)["tool_use_id"])
			matches := candidates[id]
			if len(matches) > 1 {
				return nil, fmt.Errorf("ambiguous Claude tool result children")
			}
			if len(matches) == 1 && matches[0] == i && !added[i] {
				children[owners[id]] = append(children[owners[id]], i)
				added[i] = true
			}
		}
	}
	out := make([]int, 0, len(selected)+len(added))
	for _, i := range selected {
		out = append(out, i)
		out = append(out, children[i]...)
	}
	return out, nil
}

type objectKey int

// activeBranch follows Claude UUID ancestry, not append order. Inactive forks
// and subagent records never become conversation context.
func activeBranch(records []object) ([]int, error) {
	byID := map[string]int{}
	var candidates []int
	leaf := ""
	sidechains := false
	for i, row := range records {
		if id := str(row["uuid"]); id != "" {
			if _, ok := byID[id]; ok {
				return nil, fmt.Errorf("duplicate Claude record UUID")
			}
			byID[id] = i
		}
		if row["type"] == "last-prompt" && str(row["leafUuid"]) != "" {
			leaf = str(row["leafUuid"])
		}
		if (row["type"] == "user" || row["type"] == "assistant") && obj(row["message"]) != nil && row["isMeta"] != true {
			if row["isSidechain"] == true {
				sidechains = true
			} else {
				candidates = append(candidates, i)
			}
		}
	}
	if len(candidates) == 0 {
		if sidechains {
			return nil, fmt.Errorf("sidechain/subagent transcripts cannot be imported; use the parent session")
		}
		return nil, fmt.Errorf("no Claude conversation records")
	}
	if leaf == "" {
		leaf = str(records[candidates[len(candidates)-1]]["uuid"])
	}
	if leaf == "" {
		// Old, UUID-less transcripts are only accepted if the entire conversation is
		// UUID-less. Do not flatten a partially specified graph.
		for _, i := range candidates {
			if str(records[i]["uuid"]) != "" || str(records[i]["parentUuid"]) != "" {
				return nil, fmt.Errorf("incomplete Claude UUID graph")
			}
		}
		return candidates, nil
	}
	seen := map[string]bool{}
	var reversed []int
	for leaf != "" {
		if seen[leaf] {
			return nil, fmt.Errorf("Claude ancestry cycle")
		}
		i, ok := byID[leaf]
		if !ok {
			return nil, fmt.Errorf("Claude graph references a missing UUID")
		}
		seen[leaf] = true
		reversed = append(reversed, i)
		row := records[i]
		leaf = str(row["parentUuid"])
		if leaf == "" && row["type"] == "system" && row["subtype"] == "compact_boundary" {
			leaf = str(row["logicalParentUuid"])
			if seen[leaf] && preservedBackEdge(row, leaf, seen, byID, records) {
				break
			}
		}
	}
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed, nil
}

func preservedBackEdge(boundary object, logical string, seen map[string]bool, ids map[string]int, records []object) bool {
	meta := obj(boundary["compactMetadata"])
	segment, messages := obj(meta["preservedSegment"]), obj(meta["preservedMessages"])
	anchor, head, tail := str(segment["anchorUuid"]), str(segment["headUuid"]), str(segment["tailUuid"])
	if anchor == "" || head == "" || tail == "" || logical != tail {
		return false
	}
	i, ok := ids[anchor]
	if !ok || records[i]["isCompactSummary"] != true || records[i]["parentUuid"] != boundary["uuid"] {
		return false
	}
	declared, ok := messages["allUuids"].([]any)
	if !ok {
		declared, _ = messages["uuids"].([]any)
	}
	hasHead, hasTail := false, false
	for _, v := range declared {
		id := str(v)
		if id == "" || !seen[id] {
			return false
		}
		hasHead = hasHead || id == head
		hasTail = hasTail || id == tail
	}
	if !hasHead || !hasTail {
		return false
	}
	path := map[string]bool{}
	cursor := tail
	for cursor != "" && cursor != anchor {
		i, ok := ids[cursor]
		if !ok || path[cursor] || !seen[cursor] {
			return false
		}
		path[cursor] = true
		cursor = str(records[i]["parentUuid"])
	}
	return cursor == anchor && path[head]
}

func portableImage(b object) object {
	source := obj(b["source"])
	mime, data := str(source["media_type"]), str(source["data"])
	if source["type"] != "base64" || (mime != "image/png" && mime != "image/jpeg" && mime != "image/gif" && mime != "image/webp") || data == "" {
		return nil
	}
	if _, err := base64.StdEncoding.DecodeString(data); err != nil {
		return nil
	}
	return object{"type": "image", "mimeType": mime, "data": data}
}

func portableResult(v any, report *Report) []object {
	out := []object{}
	var blocks []any
	switch value := v.(type) {
	case string:
		blocks = []any{object{"type": "text", "text": value}}
	case []any:
		blocks = value
	default:
		report.Omitted["tool_result_content"]++
		return out
	}
	for _, value := range blocks {
		if text, ok := value.(string); ok {
			out = append(out, object{"type": "text", "text": text})
			continue
		}
		b := obj(value)
		if b["type"] == "text" {
			if text, ok := b["text"].(string); ok {
				out = append(out, object{"type": "text", "text": text})
				continue
			}
		}
		if b["type"] == "image" {
			if image := portableImage(b); image != nil {
				out = append(out, image)
				report.Preserved["images"]++
				continue
			}
		}
		report.Omitted["tool_result_block"]++
	}
	return out
}

func contentText(v any) string {
	if text, ok := v.(string); ok {
		return text
	}
	var text bytes.Buffer
	if blocks, ok := v.([]any); ok {
		for _, value := range blocks {
			b := obj(value)
			if b["type"] == "text" {
				if text.Len() > 0 {
					text.WriteByte('\n')
				}
				text.WriteString(str(b["text"]))
			}
		}
	}
	return text.String()
}

func emptyUsage() object {
	return object{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "totalTokens": 0, "cost": object{"input": 0, "output": 0, "cacheRead": 0, "cacheWrite": 0, "total": 0}}
}
