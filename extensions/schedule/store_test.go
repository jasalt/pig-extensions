// Copyright (c) 2026 Jarkko Saltiola
// SPDX-License-Identifier: MIT

package schedule

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixedNow = time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

func testStore(t *testing.T) (store, string) {
	t.Helper()
	root := t.TempDir()
	return store{paths: newPaths(filepath.Join(root, "home")), now: func() time.Time { return fixedNow }}, root
}

func newTestJob(id, scope, project string) Job {
	job := testJob(Spec{Type: "interval", EveryMs: msHour, Every: "1h"}, isoTime(fixedNow))
	job.ID, job.Scope, job.ProjectPath = id, scope, project
	return job
}

func TestStoreCreateListUpdate(t *testing.T) {
	s, root := testStore(t)
	projectA, projectB := filepath.Join(root, "a"), filepath.Join(root, "b")
	if _, err := s.create(newTestJob("g1", scopeGlobal, ""), projectA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.create(newTestJob("pa", scopeProject, projectA), projectA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.create(newTestJob("pb", scopeProject, projectB), projectB); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.list(projectA)
	if err != nil || len(jobs) != 2 || jobs[0].ID != "g1" || jobs[1].ID != "pa" || jobs[1].ProjectPath != projectA {
		t.Fatalf("%+v %v", jobs, err)
	}
	if info, _ := os.Stat(s.paths.globalFile); info.Mode().Perm() != 0o600 {
		t.Fatal("store file must be private")
	}
	shell := &ShellResult{OK: true, Command: "x", Code: 0, Stdout: "out"}
	at := fixedNow.Add(time.Minute)
	updated, err := s.markAttempt(jobs[0], projectA, at, statusOK, "", "k1", true, shell)
	if err != nil || updated.RunCount != 1 || updated.NextRunAt != isoTime(at.Add(time.Hour)) || deref(updated.LastStatus) != statusOK ||
		updated.LastIdempotencyKey != "k1" || updated.LastShell.Stdout != "out" || *updated.LastRunAt != isoTime(at) {
		t.Fatalf("%+v %v", updated, err)
	}
	same, _ := s.markAttempt(*updated, projectA, at, statusSkipped, "why", "", false, nil)
	if same.RunCount != 1 || same.NextRunAt != updated.NextRunAt || same.LastError != "why" || same.LastIdempotencyKey != "k1" {
		t.Fatalf("skip without advance: %+v", same)
	}
	terminated, _ := s.terminate(*same, projectA, "maxRuns", at)
	if terminated.Enabled || deref(terminated.Terminated) != "maxRuns" {
		t.Fatal(terminated)
	}
	disabled, _ := s.setEnabled(*terminated, projectA, false)
	if deref(disabled.Terminated) != "maxRuns" {
		t.Fatal("disable keeps terminated")
	}
	enabled, _ := s.setEnabled(*disabled, projectA, true)
	if !enabled.Enabled || enabled.Terminated != nil {
		t.Fatal("enable clears terminated")
	}
	if removed, _ := s.remove("pa", projectA); removed == nil || removed.ID != "pa" {
		t.Fatal(removed)
	}
	if removed, _ := s.remove("pb", projectA); removed != nil {
		t.Fatal("another project's job is not visible from this cwd")
	}
	if missing, _ := s.setEnabled(newTestJob("nope", scopeGlobal, ""), projectA, true); missing != nil {
		t.Fatal("setEnabled on a missing id")
	}
}

// Reproduces stale-attempts-preserve-disable-and-never-resurrect-cancelled-jobs.
func TestStaleAttemptsPreserveDisableAndNeverResurrect(t *testing.T) {
	s, root := testStore(t)
	job := newTestJob("j", scopeGlobal, "")
	if _, err := s.create(job, root); err != nil {
		t.Fatal(err)
	}
	stale := job // a runner's copy taken before the concurrent change
	fresh, _ := s.get("j", root)
	if _, err := s.setEnabled(*fresh, root, false); err != nil {
		t.Fatal(err)
	}
	updated, err := s.markAttempt(stale, root, fixedNow, statusOK, "", "k", true, nil)
	if err != nil || updated == nil || updated.Enabled || updated.RunCount != 1 {
		t.Fatalf("stale completion must keep the disable: %+v %v", updated, err)
	}
	if _, err := s.remove("j", root); err != nil {
		t.Fatal(err)
	}
	for _, op := range []func() (*Job, error){
		func() (*Job, error) { return s.markAttempt(stale, root, fixedNow, statusError, "x", "k", true, nil) },
		func() (*Job, error) { return s.terminate(stale, root, "once", fixedNow) },
		func() (*Job, error) { return s.setEnabled(stale, root, true) },
	} {
		if got, err := op(); err != nil || got != nil {
			t.Fatalf("cancelled job resurrected: %+v %v", got, err)
		}
	}
	if jobs, _ := s.list(root); len(jobs) != 0 {
		t.Fatalf("%+v", jobs)
	}
}

func TestProvenanceAndForeignRows(t *testing.T) {
	s, root := testStore(t)
	file := projectFile(root)
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	rows := fmt.Sprintf(`{"version":1,"jobs":[
		{"id":"relabel","name":"x","scope":"global","action":"shell","command":"rm -rf /tmp/x","schedule":{"type":"interval","everyMs":3600000,"every":"1h"},"enabled":true,"nextRunAt":"%s"},
		{"id":"other","name":"o","projectPath":"/elsewhere","schedule":{"type":"interval","everyMs":3600000,"every":"1h"},"enabled":true,"nextRunAt":"%s"},
		{"id":"weird","name":7,"prompt":["x"],"projectPath":42,"action":"cron","wakeOn":"sometimes","timeoutMs":-1,"maxRuns":1.5,"tier":"root","missedWindow":"later","schedule":{"type":"daily","hour":30,"minute":0},"enabled":"yes","nextRunAt":"%s"},
		{"id":"long","name":"%s","prompt":"%s","command":"%s","schedule":{"type":"once","delayMs":60000,"delay":"1m"},"enabled":true,"nextRunAt":"bad"}
	]}`, isoTime(fixedNow), isoTime(fixedNow), isoTime(fixedNow), strings.Repeat("n", 300), strings.Repeat("p", 25000), strings.Repeat("c", 12000))
	_ = os.WriteFile(file, []byte(rows), 0o600)
	jobs, err := s.list(root)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("%+v %v", jobs, err)
	}
	if jobs[0].Scope != scopeProject || jobs[0].ProjectPath != absPath(root) {
		t.Fatal("rows in a project file are project jobs regardless of label", jobs[0])
	}
	weird := jobs[1]
	if weird.Name != "unnamed" || weird.Prompt != "" || weird.Action != kindPrompt || weird.WakeOn != "" || weird.TimeoutMs != 0 || weird.MaxRuns != 0 ||
		weird.Tier != tierReadOnly || weird.MissedWindow != missedCatchUp || weird.Schedule.Type != "" || weird.Enabled {
		t.Fatalf("coercion: %+v", weird)
	}
	long := jobs[2]
	if jsLength(long.Name) != maxNameChars || jsLength(long.Prompt) != maxPromptChars || jsLength(long.Command) != maxCommandChars {
		t.Fatal("clamping")
	}
	due, _ := s.due(root, fixedNow.Add(time.Hour))
	if len(due) != 1 || due[0].ID != "relabel" {
		t.Fatalf("invalid schedule/nextRunAt/enabled rows are never due: %+v", due)
	}
}

func TestCorruptStoresAreQuarantined(t *testing.T) {
	for _, body := range []string{"{not json", "null", "42", `{"version":2,"jobs":[]}`, `{"version":1,"jobs":{}}`, `{"version":1,"jobs":[null]}`, `{"version":1,"jobs":["x"]}`} {
		s, root := testStore(t)
		_ = os.MkdirAll(filepath.Dir(s.paths.globalFile), 0o700)
		_ = os.WriteFile(s.paths.globalFile, []byte(body), 0o600)
		_, err := s.list(root)
		var se storeError
		if !errors.As(err, &se) || !strings.Contains(err.Error(), "quarantined to schedules.json.corrupt-") {
			t.Fatalf("%q: %v", body, err)
		}
		if _, err := os.Stat(s.paths.globalFile); !os.IsNotExist(err) {
			t.Fatal("corrupt file must be moved aside, not left or wiped")
		}
		matches, _ := filepath.Glob(s.paths.globalFile + ".corrupt-*")
		if raw, _ := os.ReadFile(matches[0]); string(raw) != body {
			t.Fatal("quarantine preserves the original bytes")
		}
	}
	s, root := testStore(t)
	_ = os.MkdirAll(filepath.Dir(s.paths.globalFile), 0o700)
	_ = os.WriteFile(s.paths.globalFile, []byte(" \n"), 0o600)
	if jobs, err := s.list(root); err != nil || len(jobs) != 0 {
		t.Fatal("blank store is empty, not corrupt", err)
	}
}

func TestStoreLocksAndCaps(t *testing.T) {
	s, root := testStore(t)
	for i := range maxJobsPerScope {
		if _, err := s.create(newTestJob(fmt.Sprint(i), scopeGlobal, ""), root); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.create(newTestJob("over", scopeGlobal, ""), root); err == nil || !strings.Contains(err.Error(), "Job limit reached (50 per global scope)") {
		t.Fatal(err)
	}
	// A fresh lock held by another writer is never stolen.
	lock := s.paths.globalFile + ".lock"
	_ = os.WriteFile(lock, []byte(`{"pid":1,"token":"other","at":"x"}`), 0o600)
	start := time.Now()
	if _, err := s.remove("0", root); err == nil || !strings.Contains(err.Error(), "Could not acquire store lock") {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("lock retries are bounded")
	}
	if raw, _ := os.ReadFile(lock); !strings.Contains(string(raw), "other") {
		t.Fatal("fresh lock stolen")
	}
	// A stale lock (crashed session) is taken over.
	old := time.Now().Add(-time.Minute)
	_ = os.Chtimes(lock, old, old)
	if removed, err := s.remove("0", root); err != nil || removed == nil {
		t.Fatal(removed, err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("lock released after use")
	}
}

func TestConcurrentWritersDoNotLoseRows(t *testing.T) {
	s, root := testStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := store{paths: s.paths, now: s.now} // separate instances, shared files
			if _, err := other.create(newTestJob(fmt.Sprint("c", i), scopeGlobal, ""), root); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if jobs, _ := s.list(root); len(jobs) != 8 {
		t.Fatalf("lost rows: %d", len(jobs))
	}
}

func TestDefaultScope(t *testing.T) {
	dir := t.TempDir()
	if defaultScope(dir) != scopeGlobal {
		t.Fatal("no .pig")
	}
	_ = os.Mkdir(filepath.Join(dir, ".pig"), 0o700)
	if defaultScope(dir) != scopeProject {
		t.Fatal(".pig dir")
	}
	_ = os.Mkdir(filepath.Join(dir, ".pi"), 0o700)
	child := filepath.Join(dir, "child")
	_ = os.Mkdir(child, 0o700)
	if defaultScope(child) != scopeGlobal {
		t.Fatal("no upward walk")
	}
}

func TestTrust(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trusted.json")
	trust := trustStore{path: path}
	project := t.TempDir()
	if trust.isTrusted(project) {
		t.Fatal("missing file is untrusted")
	}
	if err := trust.trust(project, fixedNow); err != nil || !trust.isTrusted(project) || !(trustStore{path: path}).isTrusted(project+"/.") {
		t.Fatal("trust persists and normalizes", err)
	}
	if trust.isTrusted(t.TempDir()) {
		t.Fatal("roots are independent")
	}
	_ = trust.trust(project, fixedNow.Add(time.Hour))
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), isoTime(fixedNow.Add(time.Hour))) {
		t.Fatal("refresh trustedAt")
	}
	for _, body := range []string{"", "{bad", `{"version":2,"projects":{}}`, fmt.Sprintf(`{"version":1,"projects":{%q:"yes"}}`, absPath(project)), `{"version":1,"projects":null}`} {
		_ = os.WriteFile(path, []byte(body), 0o600)
		if trust.isTrusted(project) {
			t.Fatalf("%q must fail closed", body)
		}
	}
	if err := trust.trust(project, fixedNow); err != nil || !trust.isTrusted(project) {
		t.Fatal("a corrupt registry is rewritten on the next trust", err)
	}
}

func TestJobLocks(t *testing.T) {
	dir := t.TempDir()
	now := fixedNow
	a := newJobLocks(dir, func() time.Time { return now })
	b := newJobLocks(dir, func() time.Time { return now })
	release := a.tryAcquire("job/1")
	if release == nil || a.tryAcquire("job/1") != nil || b.tryAcquire("job/1") != nil {
		t.Fatal("single flight in-process and across instances")
	}
	if other := a.tryAcquire("job2"); other == nil {
		t.Fatal("different ids are independent")
	} else {
		other()
	}
	release()
	release() // idempotent
	if again := b.tryAcquire("job/1"); again == nil {
		t.Fatal("released lock is reusable")
	} else {
		now = fixedNow.Add(31 * time.Minute)
		if stolen := a.tryAcquire("job/1"); stolen == nil {
			t.Fatal("stale lock is taken over")
		} else {
			again() // the victim's release must not unlink the new owner's lock
			if b.tryAcquire("job/1") != nil {
				t.Fatal("victim release removed the new owner's lock")
			}
			stolen()
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "garbage.lock"), []byte("not json"), 0o600)
	if a.tryAcquire("garbage") != nil {
		t.Fatal("unreadable lock blocks")
	}
}

func TestLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "runs.jsonl")
	l := ledger{path: path, maxBytes: maxLedgerBytes}
	if !l.append(Run{RunID: "1", JobID: "a", IdempotencyKey: "k1", Status: "delivered"}) ||
		!l.append(Run{RunID: "2", JobID: "b", IdempotencyKey: "k2", Status: "skipped"}) {
		t.Fatal("append")
	}
	if !l.wasDelivered("k1") || l.wasDelivered("k2") || l.wasDelivered("k3") {
		t.Fatal("wasDelivered")
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("{corrupt\n")
	_ = f.Close()
	if h := l.history("", 10); len(h) != 2 || h[0].RunID != "2" {
		t.Fatalf("newest first, corrupt skipped: %+v", h)
	}
	if h := l.history("a", 10); len(h) != 1 || h[0].RunID != "1" {
		t.Fatal(h)
	}
	if (ledger{path: t.TempDir()}).append(Run{}) {
		t.Fatal("unwritable ledger must report false, not panic")
	}
	for i := range maxHistory {
		l.append(Run{RunID: fmt.Sprint("r", i), IdempotencyKey: "filler", Status: "delivered"})
	}
	if l.wasDelivered("k1") {
		t.Fatal("ledger idempotency only covers the recent window")
	}
	small := ledger{path: filepath.Join(t.TempDir(), "runs.jsonl"), maxBytes: 4000}
	for i := range 100 {
		small.append(Run{RunID: fmt.Sprintf("%03d", i), Detail: strings.Repeat("é", 20), Status: "delivered"})
	}
	info, _ := os.Stat(small.path)
	if info.Size() > 4000 {
		t.Fatal("rotation bounds bytes", info.Size())
	}
	if h := small.history("", 1); h[0].RunID != "099" {
		t.Fatal("rotation keeps the newest lines")
	}
	raw, _ := os.ReadFile(small.path)
	for _, line := range nonEmptyLines(string(raw)) {
		var run Run
		if json.Unmarshal([]byte(line), &run) != nil {
			t.Fatal("rotation leaves whole lines")
		}
	}
	huge := ledger{path: filepath.Join(t.TempDir(), "runs.jsonl"), maxBytes: 10}
	huge.append(Run{RunID: "big", Detail: strings.Repeat("x", 100)})
	if raw, _ := os.ReadFile(huge.path); len(raw) != 0 {
		t.Fatal("an oversized single row rotates to empty")
	}
}
