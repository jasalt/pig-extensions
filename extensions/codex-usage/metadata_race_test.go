package codexusage

import (
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestReplacementWhileMetadataIsPendingCannotStartOldWorkInNewEpoch(t *testing.T) {
	var requests atomic.Int32
	client := newClient()
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { requests.Add(1); return nil, errors.New("must not send") })
	o := newOwner(client, time.Hour)
	defer o.shutdown()
	rec := &recordedView{keyValue: "old-session/model"}
	v := rec.view()
	entered, release := make(chan struct{}), make(chan struct{})
	v.key = func() string { close(entered); <-release; return "old-session/model" }
	o.attach(v, true)
	pending := job(func() { o.refresh(v, true) })
	waitClosed(t, entered)
	o.attach((&recordedView{keyValue: "new-session/model"}).view(), true)
	close(release)
	waitClosed(t, pending)
	if requests.Load() != 0 {
		t.Fatal("obsolete metadata promoted into new epoch")
	}
	if lines, notices := rec.output(); len(lines) != 0 || len(notices) != 0 {
		t.Fatal(lines, notices)
	}
}

func TestAbsentModelFailureIsOneNativeErrorWithNoRequest(t *testing.T) {
	var requests atomic.Int32
	client := newClient()
	client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { requests.Add(1); return nil, errors.New("must not send") })
	o := newOwner(client, time.Hour)
	defer o.shutdown()
	rec := &recordedView{keyValue: "session/no-model"}
	v := rec.view()
	v.connection = func() (connection, error) { return connection{}, errors.New("No model selected") }
	o.attach(v, true)
	o.refresh(v, true)
	lines, notices := rec.output()
	if len(lines) != 1 || lines[0] != "" || len(notices) != 1 || notices[0] != (notice{"No model selected", "error"}) || requests.Load() != 0 {
		t.Fatal(lines, notices, requests.Load())
	}
}
