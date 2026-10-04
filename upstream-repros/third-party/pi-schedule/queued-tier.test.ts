/**
 * pi-schedule v0.4.0 (ed5ea93): a read_only job's turn runs with the tier of a
 * job queued after it. Copy to test/ and run `npx vitest run test/queued-tier.test.ts`.
 *
 * In one wave, ScheduleRunner delivers job A (first send) and job B (followUp)
 * and calls privilege.enter() after each send (runner.ts:304-309, 726), before
 * either agent turn has run. tool_call consults the top of the stack
 * (privilege.ts:111), so A's tools are checked against B's tier until the one
 * agent_settled for the whole run (privilege.ts:195).
 *
 * NOT EXECUTED by the reporter (no Node available); written against the
 * pinned source and the setup() pattern of test/privilege.test.ts.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { describe, expect, it } from "vitest";
import { PrivilegeGuard } from "../src/privilege.js";

function setup() {
  let toolCall: ((e: { toolName: string; input?: unknown }) => Promise<{ block?: boolean } | undefined>) | null = null;
  const pi = {
    on(event: string, handler: never) {
      if (event === "tool_call") toolCall = handler;
    },
  } as unknown as ExtensionAPI;
  const guard = new PrivilegeGuard();
  guard.attach(pi);
  return { guard, call: (toolName: string) => toolCall!({ toolName }) };
}

describe("queued scheduled turns", () => {
  it("job A (read_only) must not run bash while job B (mutate) is queued behind it", async () => {
    const { guard, call } = setup();
    guard.enter("read_only"); // job A delivered, its turn starts
    guard.enter("mutate"); // job B delivered as followUp in the same wave
    // The agent is still executing job A's turn:
    const res = await call("bash");
    expect(res?.block).toBe(true); // FAILS on v0.4.0: undefined (allowed)
  });
});
