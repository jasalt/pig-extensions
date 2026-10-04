/**
 * pi-schedule v0.4.0 (ed5ea93): a delivery that completes after a concurrent
 * cancel resurrects the job, and one that completes after a concurrent
 * disable re-enables it. Copy to test/ and run with vitest.
 *
 * markAttempt() spreads the runner's stale copy (store.ts:487-505) and
 * upsert() re-inserts a row that is no longer present (store.ts:469).
 *
 * NOT EXECUTED by the reporter (no Node available).
 */
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { parseSchedule } from "../src/schedule.js";
import { ScheduleStore, defaultPaths } from "../src/store.js";

const temps: string[] = [];
afterEach(() => temps.splice(0).forEach((t) => rmSync(t, { recursive: true, force: true })));

function fixture() {
  const root = mkdtempSync(join(tmpdir(), "pi-schedule-stale-"));
  temps.push(root);
  const store = new ScheduleStore(defaultPaths(join(root, "home")));
  const project = join(root, "project");
  const job = store.create({ name: "j", prompt: "p", schedule: parseSchedule("every 1h"), scope: "global" });
  return { store, project, job };
}

describe("stale run completion", () => {
  it("does not resurrect a job cancelled while it was running", () => {
    const { store, project, job } = fixture(); // `job` = runner's copy at fire time
    store.remove(job.id, project); // user cancels during delivery
    store.markAttempt(job, new Date(), "ok");
    expect(store.listForCwd(project)).toHaveLength(0); // FAILS: job is back
  });

  it("keeps a disable made while the job was running", () => {
    const { store, project, job } = fixture();
    store.setEnabled(job.id, project, false);
    store.markAttempt(job, new Date(), "ok");
    expect(store.get(job.id, project)?.enabled).toBe(false); // FAILS: true
  });
});
