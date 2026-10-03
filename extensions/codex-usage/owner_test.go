package codexusage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type notice struct{ Text, Level string }
type recordedView struct {
	mu       sync.Mutex
	keyValue string
	c        connection
	lines    []string
	notices  []notice
}

func (r *recordedView) view() view {
	return view{
		connection: func() (connection, error) { r.mu.Lock(); defer r.mu.Unlock(); return r.c, nil },
		key:        func() string { r.mu.Lock(); defer r.mu.Unlock(); return r.keyValue },
		done:       func() <-chan struct{} { return nil }, err: func() error { return nil },
		status: func(s string) { r.mu.Lock(); defer r.mu.Unlock(); r.lines = append(r.lines, s) },
		notify: func(s, l string) { r.mu.Lock(); defer r.mu.Unlock(); r.notices = append(r.notices, notice{s, l}) },
	}
}
func (r *recordedView) output() ([]string, []notice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...), append([]notice(nil), r.notices...)
}
func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("owned work did not finish")
	}
}
func job(f func()) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	return done
}

func TestOverlappingRefreshCancelsOlderAndNeverPublishesItsError(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(started)
			<-r.Context().Done()
			close(cancelled)
			return
		}
		if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	rec := &recordedView{keyValue: "session/model", c: adapterConnection(t, server.URL)}
	o := newOwner(newClient(), time.Hour)
	defer o.shutdown()
	v := rec.view()
	o.attach(v, true)
	old := job(func() { o.refresh(v, true) })
	waitClosed(t, started)
	o.refresh(v, true)
	waitClosed(t, old)
	waitClosed(t, cancelled)
	lines, notices := rec.output()
	if len(lines) != 1 || len(notices) != 1 || notices[0].Level != "info" || !strings.HasPrefix(lines[0], "76.5%/5h") {
		t.Fatal(lines, notices)
	}
}
func TestEpochReplacementCancelsPossiblyAcceptedResetWithoutRetryOrLateUI(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	var posts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			close(started)
			<-r.Context().Done()
			close(cancelled)
			return
		}
		if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	rec := &recordedView{keyValue: "old-session/model", c: adapterConnection(t, server.URL)}
	fresh := &recordedView{keyValue: "new-session/model", c: adapterConnection(t, server.URL)}
	o := newOwner(newClient(), time.Hour)
	defer o.shutdown()
	o.attach(rec.view(), true)
	pending := job(func() { o.reset(rec.view(), "dummy-id") })
	waitClosed(t, started)
	o.attach(fresh.view(), true)
	waitClosed(t, pending)
	waitClosed(t, cancelled)
	if lines, notices := rec.output(); len(lines) != 0 || len(notices) != 0 {
		t.Fatal("obsolete UI", lines, notices)
	}
	o.refresh(fresh.view(), true)
	if lines, notices := fresh.output(); len(lines) != 1 || len(notices) != 1 {
		t.Fatal(lines, notices)
	}
	if posts.Load() != 1 {
		t.Fatal("retried redemption")
	}
}
func TestShutdownCancelsAndJoinsInflightWork(t *testing.T) {
	started, cancelled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done(); close(cancelled) }))
	defer server.Close()
	rec := &recordedView{keyValue: "session/model", c: adapterConnection(t, server.URL)}
	o := newOwner(newClient(), time.Hour)
	o.attach(rec.view(), true)
	pending := job(func() { o.refresh(rec.view(), true) })
	waitClosed(t, started)
	waitClosed(t, job(o.shutdown))
	waitClosed(t, pending)
	waitClosed(t, cancelled)
	o.refresh(rec.view(), true)
	o.reset(rec.view(), "dummy-id")
	if lines, notices := rec.output(); len(lines) != 0 || len(notices) != 0 {
		t.Fatal("post shutdown UI", lines, notices)
	}
}
func TestAutomaticFailuresAreSilentAndClearOnlyOwnedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = io.WriteString(w, "denied dummy-secret-token")
	}))
	defer server.Close()
	rec := &recordedView{keyValue: "session/model", c: adapterConnection(t, server.URL)}
	o := newOwner(newClient(), time.Hour)
	defer o.shutdown()
	o.attach(rec.view(), true)
	o.refresh(rec.view(), false)
	lines, notices := rec.output()
	if len(lines) != 1 || lines[0] != "" || len(notices) != 0 {
		t.Fatal(lines, notices)
	}
	o.refresh(rec.view(), true)
	lines, notices = rec.output()
	if len(lines) != 2 || len(notices) != 1 || notices[0].Level != "error" || strings.Contains(notices[0].Text, "dummy-secret-token") {
		t.Fatal(lines, notices)
	}
}
func TestTimerOwnsRuntimeNotNormalHandlerCompletionAndStopsOnDisconnect(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	rec := &recordedView{keyValue: "session/model", c: adapterConnection(t, server.URL)}
	v := rec.view()
	normal := make(chan struct{})
	runtime, cancelRuntime := context.WithCancel(context.Background())
	defer cancelRuntime()
	var completed atomic.Bool
	v.done = func() <-chan struct{} {
		if completed.Load() {
			return runtime.Done()
		}
		return normal
	}
	v.err = func() error { return runtime.Err() }
	o := newOwner(newClient(), 10*time.Millisecond)
	defer o.shutdown()
	o.attach(v, true)
	o.start(v)
	completed.Store(true)
	close(normal)
	deadline := time.After(3 * time.Second)
	for requests.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("poller stopped on normal response completion")
		case <-time.After(time.Millisecond):
		}
	}
	cancelRuntime()
	waitClosed(t, job(func() { o.background.Wait() }))
	before := requests.Load()
	time.Sleep(30 * time.Millisecond)
	if requests.Load() != before {
		t.Fatal("ticker survived disconnect")
	}
	if refreshInterval != 5*time.Minute {
		t.Fatal(refreshInterval)
	}
	if _, notices := rec.output(); len(notices) != 0 {
		t.Fatal("automatic transcript spam", notices)
	}
}
func TestLiveModelKeyRejectsLatePublicationEvenBeforeReplacementEvent(t *testing.T) {
	rec := &recordedView{keyValue: "session/old-model"}
	o := newOwner(newClient(), time.Hour)
	defer o.shutdown()
	v := rec.view()
	op, ok := o.begin(v, true)
	if !ok {
		t.Fatal("begin")
	}
	rec.mu.Lock()
	rec.keyValue = "session/new-model"
	rec.mu.Unlock()
	published := o.publish(op, v, func() { t.Error("stale result published") })
	op.cancel()
	o.jobs.Done()
	if published {
		t.Fatal("model-key guard bypassed")
	}
}
func TestImmediateResetAndRefreshWithNoConfirmationOrAgentTurn(t *testing.T) {
	var posts, gets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posts.Add(1)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		gets.Add(1)
		if err := json.NewEncoder(w).Encode(testPayload()); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	rec := &recordedView{keyValue: "session/model", c: adapterConnection(t, server.URL)}
	o := newOwner(newClient(), time.Hour)
	defer o.shutdown()
	o.attach(rec.view(), true)
	o.reset(rec.view(), " exact-id ")
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatal(posts.Load(), gets.Load())
	}
	lines, notices := rec.output()
	if len(lines) != 1 || len(notices) != 1 || notices[0] != (notice{"Reset exact-id activated (0 rate-limit windows reset).", "info"}) {
		t.Fatal(lines, notices)
	}
	o.reset(rec.view(), "not an id")
	if posts.Load() != 1 || gets.Load() != 1 {
		t.Fatal("invalid ID sent")
	}
	_, notices = rec.output()
	if notices[len(notices)-1] != (notice{"Usage: /codex-reset [reset-id]", "error"}) {
		t.Fatal(notices)
	}
}
