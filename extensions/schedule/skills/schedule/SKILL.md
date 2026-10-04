---
name: schedule
description: 'Schedule recurring agent tasks and actions — POLLS (CI / build / deploy / test / PR status), periodic checks, security reviews, version checks, reminders, scans. Owns the `schedule` tool (create / list / cancel / enable / disable / run_now / history / trust) AND the design choices: kind (prompt/shell/notify/message), every-vs-dailyAt-vs-once, tier (read_only/suggest/mutate), wakeOn for shell, missed-window policy, project-vs-global scope, and how to write a self-contained prompt. Use when the user wants to POLL, WATCH, or CHECK something repeatedly — "poll the CI/build/deploy/tests", "check every N minutes", "check periodically", "watch for", "keep an eye on", "monitor", "ping me when", "alert me if", "remind me", "every day", "hourly", "daily at", "schedule", "recurring", "periodic", "cron" — or wants automated / repeating agent work.'
---

# Schedule — recurring agent tasks

This skill drives the **`schedule`** tool from the PiG `schedule` extension
(a Go port of pi-schedule v0.4.0). The tool stores and fires jobs; the value
comes from choosing parameters well and writing a prompt that works on its own.

## How firing works (read this first)

What fires depends on **`kind`**:
- **prompt** — a scheduled agent turn (below)
- **shell** — runs `command` first; agent turn only if `wakeOn` says so
- **notify** / **message** — human-visible only; **no** agent turn

For **prompt** (and shell wakes), the job is submitted as a **new user message
in the current session**. It is *not* a fresh conversation: the session's earlier
history is still in context. PiG prepends a header, then your instruction:

```
[scheduled-task]
runId: …  jobId: …  name: …  action: prompt  schedule: …  tier: read_only
## Task
<your prompt, verbatim>
## Contract
- This is a scheduled run. Focus only on this task.
- If tools fail or data is missing, report it; do NOT invent findings.
- If nothing is actionable, say so (e.g. "No findings").
- Prefer evidence over unsupported claims.
- Don't touch other schedules.
PRIVILEGE: read_only   →  only known read tools are allowed for this turn
```

Two consequences shape every decision below:

1. **Write the prompt so it does not depend on the conversation.** The history
   is there, but it may be about something else entirely by the time the job
   fires. No "as we discussed". State the goal, scope and expected output.
2. **Privilege is enforced structurally**, not just by wording. Tool calls in
   a scheduled turn are checked against its tier and blocked. Pick the tier by
   what the task *needs to run*. Quiet shell / notify / message fires start no
   agent turn.

Firing triggers: when a session is created or resumed (`/new`, `/resume`), and
every **30s** while the session is open and the agent is idle. At process
startup the first check waits for that idle 30s tick, so a prompt given on the
command line always runs first. There is **no background daemon**: jobs only
fire while PiG is running with this extension.

## The tool at a glance

```
schedule
  action: create | list | cancel | enable | disable | run_now | history | trust
  name          (create) short label
  kind          (create) prompt | shell | notify | message   (default prompt)
  prompt        (create) task/reminder text; optional shell follow-up
  command       (create, kind=shell) shell command via bash -lc
  wakeOn        (create, kind=shell) always | failure | success | never
  successPrompt / failurePrompt   (shell) outcome-specific agent text
  timeoutMs     (shell) default 60000, max 600000
  once          (create) one-shot delay ("10m"/"30s"), then terminate (xor every/dailyAt)
  maxRuns       (create) cap deliveries (ok+error) before auto-disable
  every         (create) "30m" | "2h" | "1d"   (xor with dailyAt/once)
  dailyAt       (create) "09:00" local time   (xor with every/once)
  scope         global | project              (default: project if .pig/ exists in cwd)
  tier          read_only | suggest | mutate  (default read_only; shell forces mutate)
  missedWindow  catch_up_one | skip           (default catch_up_one)
  id            (cancel/enable/disable/run_now/history)
  limit         (history, default 10, max 50)
```

**Interval rules:** min `1m`, max `90d`; `once` allows seconds (`30s`) up to `90d`.
**Always `list` before `create`** to avoid duplicate jobs (no auto-dedup).
**Terminated jobs** (once fired / maxRuns reached) are disabled; `enable` clears the flag, or cancel + recreate.

## Decisions

### `kind` — what should fire?

| Want | kind | Notes |
|---|---|---|
| Agent does the work (review, summarize, draft) | **prompt** (default) | Contract header + tier enforcement |
| Run a command; wake agent only sometimes | **shell** | No model tokens on quiet success; use `wakeOn=failure` for CI |
| Nudge the human only | **notify** | Notification plus a display-only session note; no agent turn |
| Drop a note into the session | **message** | Display-only session message; no agent turn |

**Shell rules:**
- Requires `command`. Always stored as `tier=mutate`.
- `wakeOn` default: `always` if any follow-up text (`prompt` / `successPrompt` / `failurePrompt`) is set, else `never`.
- Prefer **`wakeOn=failure`** for polls so green stays silent.
- Follow-up priority: `successPrompt` → `failurePrompt` → `prompt` → generic review text.
- Global shell jobs run in the session's cwd; use absolute paths or a project job for a fixed directory.
- Override the shell binary with `PIG_SCHEDULE_SHELL=/absolute/path/to/bash`.
- Persisted shell output is redacted for common credential shapes; the follow-up turn sees the full output.

Do **not** use `kind=shell` for open-ended investigation — use `kind=prompt` with `tier=mutate` so the agent chooses commands.

### Lifecycle — `once` vs recurring vs `maxRuns`

| Want | Use |
|---|---|
| Fire one time, then stop | **`once="10m"`** (relative; seconds allowed) |
| Recurring heartbeat / review | **`every`/`dailyAt`** |
| Bounded poll — stop after N checks | **`every` + `maxRuns=N`** |

`run_now` refuses a terminated job; `enable` clears the flag to resume a `maxRuns` job.

### `every` vs `dailyAt`

Polls → `every` (`30m`, `2h`, `1d`). Reports/reviews at a wall-clock time →
`dailyAt` (`09:00`), local timezone and DST-safe.

### `scope`

- **project** (default when `.pig/` exists in cwd): stored in
  `<cwd>/.pig/schedule.json`. Launch PiG from the project root. Project jobs
  only fire automatically once the project is **trusted**: creating a project
  job from a normal turn trusts it, or run `schedule action=trust`. Inspect a
  cloned project's `.pig/schedule.json` before trusting it — it can contain
  shell jobs. `run_now` is explicit and bypasses the gate.
- **global**: stored under PiG's config root (`state/schedule/`), fires in
  every session.

### `tier` — pick by what the task must *run*

| Tier | Allowed on the fired turn | Use when |
|---|---|---|
| **read_only** (default) | Known read tools only (read, grep, find, ls, …); unknown tools fail closed. No shell, no edits. | Read/search/analyze only. |
| **suggest** | Everything except shell/exec tools (`bash`, `powershell`, …) and peer messaging. | Drafting edits without running commands. |
| **mutate** | Everything | The task must change files **or run shell** (`git`, `npm`, `gh`). Use sparingly — it runs unattended. |

> ⚠️ A task that needs the **shell** (`npm outdated`, `git log`, a script)
> **must be `tier="mutate"`**. `read_only` and `suggest` both block it.

`read_only` and `suggest` turns also cannot create, cancel, enable, disable,
run or trust schedules (`list`/`history` stay allowed). Setting
`PIG_SCHEDULE_PRIVILEGE_MODE=legacy` relaxes `read_only` to blocking only
`edit`, `write`, `bash` and `powershell`.

### `missedWindow` — what happens when a fire is overdue

- **catch_up_one** (default): fire once for the missed slot, then reschedule.
- **skip**: fire only within grace (interval: `max(2×tick, 25% of period)`,
  capped at 15m; daily: 1h); otherwise roll forward **without firing**.

## Writing the prompt (the part that matters)

- **Goal in one line.**
- **Scope & inputs.** Which files/paths/commands? Don't assume context.
- **Expected output.** "List outdated packages as current→latest".
- **Constraints.** "Focus only on `src/auth/`".
- **Escape hatch.** "Reply 'No findings' if clean" — otherwise the model may fabricate.

Good: `Review src/auth/ for injection, auth bypass and exposed secrets. Cite file:line. If none, reply exactly "No findings". Do not modify anything.`

Bad: `check the thing we talked about and tell me if it's still broken`

## Recipes

```text
# Daily static review (read only)
schedule action=create name="security-review" dailyAt="09:00" scope="project" tier="read_only"
  prompt="Review the code under src/auth/ for security issues. Cite file:line. If none, reply 'No findings'. Do not modify anything."

# Poll CI every 5m; wake only on failure
schedule action=create name="ci-poll" kind="shell" every="5m" missedWindow="skip" wakeOn="failure"
  command="gh run list --limit 1 --json conclusion -q '.[0].conclusion' | grep -vq failure"
  failurePrompt="Latest CI run failed. Inspect jobs/logs and propose or apply fixes."

# Human reminder, no agent tokens
schedule action=create name="stretch" kind="notify" every="1h" prompt="Stand up and stretch for 2 minutes."

# Agent must run commands → mutate
schedule action=create name="pkg-outdated" every="1d" tier="mutate" missedWindow="skip"
  prompt="Run `npm outdated` for prod deps. Report meaningful updates as current→latest. If nothing meaningful, reply 'No findings'."
```

## Guardrails & verification

- **Limits:** 50 jobs per scope; 10 creates/minute; 5 fires per session-start
  check, 3 per tick.
- `schedule action=run_now id=<id>` fires once and reports the **actual**
  status (`ok` / `error` / `skipped`, or "did not fire").
- `schedule action=history id=<id>` — append-only run trail.
- `schedule action=list` — next/last run, run count, last status.
- `delivered` means the message or action was submitted, not that the agent
  completed the task.
- `disable` pauses a job; `cancel` deletes it.
