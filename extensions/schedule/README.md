# schedule

Recurring and one-shot agent prompts, shell checks, reminders and session
messages for PiG. Go port of [pungggi/pi-schedule](https://github.com/pungggi/pi-schedule)
v0.4.0 (`ed5ea93f1a82ac7038fda34acbbfaa572472f9e3`); upstream's MIT license is
kept in [LICENSE](LICENSE). Ships the `schedule` skill
([skills/schedule/SKILL.md](skills/schedule/SKILL.md)).

```sh
pig --no-extensions -e /absolute/path/to/pig-extensions/extensions/schedule
pig install --validate-only --json ./extensions/schedule   # build/register only
```

**Jobs fire only while a PiG session with this extension is open.** There is
no daemon or OS service.

## Tool: `schedule`

Actions: `create`, `list`, `cancel`, `enable`, `disable`, `run_now`, `history`,
`trust`. Kinds: `prompt` (default), `shell`, `notify`, `message`. Schedules:
`every` (`30m`, `2h`, `1d`; 1m–90d), `dailyAt` (`09:00`, local time,
DST-aware), or `once` (`10m`, `30s`; up to 90d). Other fields: `name`,
`prompt`, `command`, `wakeOn`, `successPrompt`, `failurePrompt`, `timeoutMs`
(default 60 s, max 10 min), `maxRuns`, `scope`, `tier`, `missedWindow`, `id`,
`limit`. See the skill for when to use each.

| Kind | When due |
| --- | --- |
| `prompt` | Submits a `[scheduled-task]` user message **into the current session** (not a fresh conversation) |
| `shell` | Runs `bash -lc <command>`; records a session message; wakes the agent per `wakeOn` |
| `notify` | Notification plus a display-only session message; no agent turn |
| `message` | Display-only session message; no agent turn |

## When jobs fire

- `/new` or `/resume`: due jobs fire immediately (max 5).
- Every 30 s while the session is open and the agent is idle (max 3 per tick).
- **At process startup the first check waits for that idle tick** (see
  differences). `run_now` fires one job immediately.
- `catch_up_one` fires once for a missed slot; `skip` fires only within grace
  (interval: max(1 min, 25% of period) capped at 15 min; daily: 1 h).
- `once` jobs and jobs reaching `maxRuns` terminate (disabled). `enable`
  clears that; `run_now` refuses a terminated job.
- `delivered` means submitted, not that the agent finished the task.

## Privilege tiers

Prompt jobs default to `read_only`; shell jobs always run as `mutate`. Tool
calls in a scheduled turn are checked against that turn's tier and blocked
with early termination of a fully blocked batch:

| Tier | Blocked |
| --- | --- |
| `read_only` (strict) | Everything except known read tools (`read`, `grep`, `find`, `ls`, `show_image`, …); unknown tools fail closed |
| `read_only` (`PIG_SCHEDULE_PRIVILEGE_MODE=legacy`) | `edit`, `write`, `bash`, `powershell`, peer messaging |
| `suggest` | `bash`, `powershell`, terminal exec tools, peer messaging |
| `mutate` | Nothing |

`read_only` and `suggest` turns also cannot run mutating `schedule` actions
(`list`/`history` stay allowed). Each delivery reserves its exact prompt text;
a tool call is restricted only when the newest user message on the active
branch is a reserved scheduled prompt. Several prompts queued in one agent run
each keep their own tier, and the user's own turns are never restricted.

Tool policy is not an OS sandbox: shell jobs and agent turns run with the PiG
process's permissions, and installed extensions are trusted code.

## Storage and trust

| Path | Content |
| --- | --- |
| `<config-root>/state/schedule/schedules.json` | Global jobs |
| `<cwd>/.pig/schedule.json` | Project jobs (default scope when `<cwd>/.pig/` exists; no upward walk) |
| `<config-root>/state/schedule/runs.jsonl` | Run ledger (rotated at 5 MiB) |
| `<config-root>/state/schedule/trusted.json` | Trusted project roots |
| `<config-root>/state/schedule/locks/` | Per-job delivery locks |
| `<config-root>/state/schedule/skills/schedule/SKILL.md` | Skill copy offered through `resources_discover` |

`<config-root>` is `PIG_HOME` (default `~/.pig`). Files are written atomically
with mode `0600`. Writes take a per-file lock and re-read the row inside it,
so a stale run cannot undo a concurrent `disable` or resurrect a cancelled
job. Corrupt stores are quarantined (`*.corrupt-<time>`), never silently
emptied. Locks and idempotency keys reduce duplicates, but this is not an
exactly-once transaction.

Project jobs auto-fire only in **trusted** projects. Creating a project job
in a normal (non-scheduled) turn trusts the project, as does
`schedule action=trust`. A scheduled turn cannot trust its own project.
Inspect a cloned `.pig/schedule.json` before trusting it: it can hold shell
jobs. Global jobs are not gated.

Shell output persisted to the store and session is redacted for common token
shapes and `KEY=value` credentials. The agent follow-up keeps full output,
and the command is stored verbatim — don't put secrets in `command`.

## Behavior differences from pi-schedule v0.4.0

- **Startup wave deferred.** The original skips the startup check only when
  Pi was launched with an initial prompt, by re-parsing the parent's argv.
  PiG gives extensions no supported signal for that (the initial prompt
  arrives as an ordinary `interactive` input), so the startup check always
  waits for the first idle 30 s tick. A command-line prompt therefore always
  runs first, and in a no-prompt launch due jobs fire up to 30 s later.
  `/new` and `/resume` are unchanged. An approved host field
  (`hasInitialPrompt` on `session_start`) would restore exact behavior.
- **Exact turn ownership.** The original pushes a tier stack and applies the
  newest tier to every tool call until a settle pops it, so a read_only
  prompt followed by a queued mutate prompt runs its tools as mutate. Here
  each turn is bound to its own prompt (see Privilege tiers).
- **Shell jobs run in their own process group** from the extension rather
  than through the host executor. On PiG 0.3.1, the host executor's timeout
  kills the shell but not a backgrounded child, and the call blocks until that
  child exits (measured: 12 s for a 1.5 s timeout). Timeout and shutdown here
  kill the whole group. Output capture keeps a bounded head and tail.
- `powershell` is treated as a shell surface in every tier.
- Fresh-row read-modify-write for all mutations and caps checked inside the
  insert lock (kmet's fix); a job disabled or cancelled mid-run stays so.
- Compaction: PiG reports a prompt rejected during compaction to the user,
  not to the extension, so delivery waits on `session_before_compact` /
  `session_compact` / `session_compact_failed` (bounded at 120 s), and
  retries a returned busy error as a backstop.
- The contract line says "scheduled run", not "isolated", and the skill no
  longer promises "no prior conversation": prompts enter the current session.
- PiG paths, `[pig-schedule]` labels, `pig-schedule` message type,
  `PIG_SCHEDULE_SHELL` / `PIG_SCHEDULE_PRIVILEGE_MODE`. No import of
  `~/.pi-schedule` or `.pi/schedule.json`. Linux/POSIX only.
- Session-start waves run in the background instead of blocking the
  `session_start` handler.

## Tests

```sh
(cd extensions/schedule && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.schedule_rpc
```
