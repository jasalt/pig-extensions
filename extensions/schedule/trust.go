// Copyright (c) 2025 Alessandro Pungitore
// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// trustStore gates automatic firing of project jobs: a cloned
// .pig/schedule.json must not auto-run shell or mutate rows. An unreadable
// or corrupt file means untrusted.
type trustStore struct{ path string }

type trustFile struct {
	Version  int                          `json:"version"`
	Projects map[string]map[string]string `json:"projects"`
}

func trustKey(root string) string {
	abs, err := filepath.Abs(root)
	if err != nil {
		return filepath.Clean(root)
	}
	return abs
}

func (t trustStore) read() (trustFile, bool) {
	raw, err := os.ReadFile(t.path)
	if err != nil || strings.TrimSpace(string(raw)) == "" {
		return trustFile{}, false
	}
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) != nil {
		return trustFile{}, false
	}
	var file struct {
		Version  int                        `json:"version"`
		Projects map[string]json.RawMessage `json:"projects"`
	}
	if json.Unmarshal(raw, &file) != nil || file.Version != 1 || file.Projects == nil {
		return trustFile{}, false
	}
	out := trustFile{Version: 1, Projects: map[string]map[string]string{}}
	for key, value := range file.Projects {
		var record map[string]any
		if json.Unmarshal(value, &record) == nil && record != nil {
			at, _ := record["trustedAt"].(string)
			out.Projects[key] = map[string]string{"trustedAt": at}
		}
	}
	return out, true
}

func (t trustStore) isTrusted(root string) bool {
	file, ok := t.read()
	if !ok {
		return false
	}
	_, trusted := file.Projects[trustKey(root)]
	return trusted
}

// trust records root (idempotent; refreshes trustedAt). A corrupt registry is
// replaced on write; gate reads stay fail-closed.
func (t trustStore) trust(root string, at time.Time) error {
	_, err := withFileLock(t.path, func() (struct{}, error) {
		file, ok := t.read()
		if !ok {
			file = trustFile{Version: 1, Projects: map[string]map[string]string{}}
		}
		file.Projects[trustKey(root)] = map[string]string{"trustedAt": isoTime(at)}
		return struct{}{}, writeJSONAtomic(t.path, file)
	})
	return err
}
