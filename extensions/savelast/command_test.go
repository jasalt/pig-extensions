package savelast

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// Exercise the real SDK request/notification boundary with a failing host
// session read. Production PiG cannot be instructed to inject this failure.
func TestCommandReportsSessionReadFailure(t *testing.T) {
	host, runner := net.Pipe()
	defer host.Close()
	defer runner.Close()
	if err := host.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Extension().RunWithConn(runner) }()
	read := func() map[string]any {
		t.Helper()
		var size uint32
		if err := binary.Read(host, binary.BigEndian, &size); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(host, data); err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		return row
	}
	write := func(row map[string]any) {
		t.Helper()
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		if err := binary.Write(host, binary.BigEndian, uint32(len(data))); err != nil {
			t.Fatal(err)
		}
		if _, err := host.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if row := read(); row["type"] != "register" {
		t.Fatalf("registration: %v", row)
	}
	write(map[string]any{"type": "ready", "ready": map[string]any{"cwd": t.TempDir(), "width": 80}})
	write(map[string]any{"type": "request", "id": "save", "request": map[string]any{"method": "command", "tool": "savelast", "args": "out.md"}})
	notified := false
	for {
		row := read()
		switch row["type"] {
		case "call":
			call := row["call"].(map[string]any)
			result := map[string]any{"result": map[string]any{}}
			switch call["method"] {
			case "sessionRead":
				result = map[string]any{"error": map[string]any{"code": "fixture_failure", "message": "read unavailable"}}
			case "ui.notify":
				args := call["args"].(map[string]any)
				if args["level"] != "error" || !strings.Contains(args["message"].(string), "Failed to read session: fixture_failure: read unavailable") {
					t.Fatalf("notification: %v", args)
				}
				notified = true
			default:
				t.Fatalf("unexpected host call: %v", call)
			}
			write(map[string]any{"type": "call_result", "id": row["id"], "call_result": result})
		case "response":
			if !notified {
				t.Fatal("no error notification")
			}
			response := row["response"].(map[string]any)
			if response["error"] != nil {
				t.Fatalf("duplicate command error: %v", response)
			}
			write(map[string]any{"type": "shutdown", "shutdown": map[string]any{"reason": "test"}})
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("runner did not stop")
			}
			return
		}
	}
}

func TestMissingContentMatchesOriginalUndefinedButNullIsTextless(t *testing.T) {
	entry := assistantEntry(nil)
	if _, found := latestAssistantText([]map[string]any{entry}); !found {
		t.Fatal("null is a present textless content")
	}
	delete(entry["message"].(map[string]any), "content")
	if _, found := latestAssistantText([]map[string]any{assistantEntry("older"), entry}); found {
		t.Fatal("missing content must not fall back")
	}
}
