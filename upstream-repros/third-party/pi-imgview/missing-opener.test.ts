/**
 * pi-imgview 17b568e: with no `xdg-open` (or `open`) on PATH, openInBrowser()
 * returns normally and the tool reports "Browser: opened <path>"
 * (index.ts:160-179). Node then emits an `error` event on the detached child
 * (utils.ts:295-299), which has no listener: an uncaught exception in the
 * host process unless it installs a handler.
 *
 * Copy to extensions/imgview/ next to utils.test.ts and run
 * `npx tsx --test extensions/imgview/missing-opener.test.ts`.
 * NOT EXECUTED by the reporter (no Node available).
 */
import * as assert from "node:assert/strict";
import { it } from "node:test";
import { openInBrowser } from "./utils.js";

it("reports a missing opener instead of claiming success", async () => {
  const savedPath = process.env.PATH;
  process.env.PATH = "/nonexistent";
  const uncaught = new Promise<unknown>((resolve) => process.once("uncaughtException", resolve));
  try {
    assert.doesNotThrow(() => openInBrowser("/tmp/imgview-probe.html")); // the caller sees success
    const error = await Promise.race([uncaught, new Promise((r) => setTimeout(() => r(null), 1000))]);
    assert.equal(error, null, `unhandled spawn error: ${String(error)}`); // FAILS: ENOENT is uncaught
  } finally {
    process.env.PATH = savedPath;
  }
});
