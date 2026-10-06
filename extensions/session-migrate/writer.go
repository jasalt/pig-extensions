// Copyright (c) 2026 Jarkko Saltiola
// Copyright (c) 2026 xhluca
// SPDX-License-Identifier: MIT
package sessionmigrate

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result describes a newly published session and its content-free manifest.
type Result struct {
	Path         string `json:"path"`
	ManifestPath string `json:"manifestPath"`
	SessionID    string `json:"sessionId"`
}

type manifest struct {
	Version             int    `json:"version"`
	Report              Report `json:"source"`
	TargetFormat        string `json:"targetFormat"`
	TargetSessionID     string `json:"targetSessionId"`
	InitialTargetSHA256 string `json:"initialTargetSHA256"`
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// WriteSession publishes a fresh PiG v3 session, without touching any existing
// session or the source. The host supplies its selected session directory.
// Both files are mode 0600; constructors and inspection never call this method.
func WriteSession(ctx context.Context, s *Transcript, dir, cwd string) (*Result, error) {
	if s == nil || len(s.Entries) == 0 {
		return nil, fmt.Errorf("no resumable context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(dir) || !filepath.IsAbs(cwd) {
		return nil, fmt.Errorf("session directory and cwd must be absolute")
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(object{"type": "session", "version": 3, "id": id, "timestamp": now.Format(time.RFC3339Nano), "cwd": cwd}); err != nil {
		return nil, err
	}
	parent := any(nil)
	appendEntry := func(draft object) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		entryID, err := newID()
		if err != nil {
			return err
		}
		entry := object{}
		for k, v := range draft {
			entry[k] = v
		}
		entry["id"], entry["parentId"] = entryID, parent
		if entry["timestamp"] == nil {
			entry["timestamp"] = now.Format(time.RFC3339Nano)
		}
		if entry["type"] == "compaction" {
			entry["firstKeptEntryId"] = entryID
		}
		if err := encoder.Encode(entry); err != nil {
			return err
		}
		parent = entryID
		return nil
	}
	for _, entry := range s.Entries {
		if err := appendEntry(entry); err != nil {
			return nil, err
		}
	}
	if s.Title != "" {
		if err := appendEntry(object{"type": "session_info", "name": strings.NewReplacer("\r", " ", "\n", " ").Replace(s.Title)}); err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256(data.Bytes())
	audit, err := json.MarshalIndent(manifest{Version: 1, Report: s.Report, TargetFormat: "pig", TargetSessionID: id, InitialTargetSHA256: hex.EncodeToString(sum[:])}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	result := &Result{Path: filepath.Join(dir, now.Format("2006-01-02T15-04-05.000Z")+"_"+id+".jsonl"), SessionID: id}
	result.ManifestPath = result.Path + ".migration.json"
	sessionTemp, err := stageFile(dir, data.Bytes())
	if err != nil {
		return nil, err
	}
	defer os.Remove(sessionTemp)
	auditTemp, err := stageFile(dir, append(audit, '\n'))
	if err != nil {
		return nil, err
	}
	defer os.Remove(auditTemp)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Hard links provide same-filesystem atomic visibility and no-replace
	// semantics. Publish the audit first, then the session's complete bytes.
	if err := os.Link(auditTemp, result.ManifestPath); err != nil {
		return nil, err
	}
	if err := os.Link(sessionTemp, result.Path); err != nil {
		os.Remove(result.ManifestPath)
		return nil, err
	}
	directory, err := os.Open(dir)
	if err == nil {
		err = directory.Sync()
		closeErr := directory.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		os.Remove(result.Path)
		os.Remove(result.ManifestPath)
		return nil, err
	}
	return result, nil
}

func stageFile(dir string, data []byte) (path string, err error) {
	f, err := os.CreateTemp(dir, ".migration-*")
	if err != nil {
		return "", err
	}
	path = f.Name()
	defer func() {
		if err != nil {
			os.Remove(path)
		}
	}()
	if _, err = f.Write(data); err != nil {
		f.Close()
		return path, err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return path, err
	}
	err = f.Close()
	return path, err
}
