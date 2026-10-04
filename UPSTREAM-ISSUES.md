# Upstream issues

Issues found while porting the kmet extensions to PiG, written so each can be
pasted into an upstream report. Nothing here has been filed; that is left to
the maintainer.

- **PiG issues** were reproduced against PiG `0.3.1`
  (`0e6ed0048282a15531ea1652a5833de4da4dd1a7`, `../PiG/bin/pig`, Go 1.27.1,
  Linux amd64). Each was also checked against upstream main at
  `e8cc487622119dceb41d80e6895d951d3a127263` by reading committed source
  (not run).
- **Third-party Pi extension issues** are from source review of the pinned
  versions. Their TypeScript repros were **not executed** (no Node.js here).

Every PiG repro is a minimal standalone extension plus script under
[`upstream-repros/`](upstream-repros/), using only a stdlib Python harness
(temporary `PIG_HOME`, no ambient extensions, local scripted model, nothing
off `127.0.0.1`):

```sh
cd upstream-repros
PIG_BIN=/path/to/pig PIG_GIT_CHECKOUT=/path/to/PiG-git-checkout python3 run_all.py
```

Last run (reviewed binary):

```text
pig-exec-timeout-process-tree: REPRODUCED — timeout 1s; call returned after 8.0s (elapsed=8.0s killed=true code=-1); background child gone after return
pig-go-sdk-terminal-capabilities: REPRODUCED — settings terminal.hyperlinks=false, terminal.images=false; extension sees hyperlinks=true images="kitty"
pig-go-sdk-overlay-height-resize: REPRODUCED — before=(1, 100, 30); frames after 30→20 rows: [(1, 100, 30)]; frames after 100→90 cols: [(2, 90, 20)]
pig-piglet-schema-extension-realization: REPRODUCED — schema buildSpec properties=['outputName', 'targets'] additionalProperties=False; validate fused -> (0, 'OK: …'); validate sometimes -> (1, 'FAIL: 1 validation error(s)')
pig-source-root-not-git: REPRODUCED — exit 1: builder native failed after execution started; no fallback was attempted: resolve Pig source revision: exit status 128
pig-rpc-extension-compact-noop: REPRODUCED — extension: ['no compaction within 5s', 'with callbacks: compaction is not available']; client compact command success=True
pig-send-during-compaction: NOT REPRODUCED — extension saw: ['sendUserMessage returned err=<nil>']; host showed compaction error: False; message reached the model: True
pig-followup-event-order: OBSERVED — ['before_agent_start:ONE', 'message_start:ONE', 'message_start:TWO', 'agent_settled']
```

| #  | Upstream              | Issue                                                                              | Severity        | On main `e8cc487` |
|----|-----------------------|------------------------------------------------------------------------------------|-----------------|-------------------|
| 1  | PiG                   | Extension `exec` timeout leaves the process tree and blocks until descendants exit | High            | Present           |
| 2  | PiG                   | Go SDK cannot see the host's resolved terminal capabilities                        | Medium          | Present           |
| 3  | PiG                   | Go SDK overlays are not re-rendered on a height-only resize                        | Low             | Present           |
| 4  | PiG                   | Piglet JSON Schema rejects `build.extensionRealization`                            | Low             | Present           |
| 5  | PiG                   | Non-git `PIG_SOURCE_ROOT` fails with `exit status 128`                             | Low             | Present           |
| 6  | PiG                   | Extension `ctx.Compact()` is a silent no-op in RPC/print mode                      | Medium          | Fixed             |
| 7  | PiG                   | No public highlighter that takes a theme                                           | Low             | Fixed             |
| 8  | pungggi/pi-schedule   | Queued scheduled turns run with a later job's privilege tier                       | High (security) | n/a               |
| 9  | pungggi/pi-schedule   | Stale run completion resurrects cancelled jobs and undoes disables                 | Medium          | n/a               |
| 10 | pungggi/pi-schedule   | Skill promises "no prior conversation"                                             | Low (docs)      | n/a               |
| 11 | gregjohnso/pi-imgview | Missing browser opener: false success and an unhandled spawn error                 | Medium          | n/a               |
| 12 | s4lv0/pi-pins         | `/pin list` documented but missing; `/pin help` opens the browser                  | Low             | n/a               |

---

## 1. PiG: extension `exec` timeout leaves the process tree and blocks until descendants exit

**Repro:** [`upstream-repros/pig-exec-timeout-process-tree`](upstream-repros/pig-exec-timeout-process-tree/)

```go
start := time.Now()
res, _ := ctx.ExecWithOptions("bash", []string{"-c", "sleep 8 & echo $! > " + pidfile + "; wait"},
	sdk.ExecOptions{Timeout: 1000})
ctx.Notify(fmt.Sprintf("elapsed=%.1fs killed=%t code=%d", time.Since(start).Seconds(), res.Killed, res.ExitCode), "info")
```

**Expected:** the call returns after about 1 s with `killed=true`, and the
command's process group (including the background `sleep`) is terminated.

**Actual:** `elapsed=8.0s killed=true code=-1`. Only `bash` is killed. The
call returns only when the background child exits on its own and closes the
inherited stdout pipe. With `sleep 600` the call blocks for ten minutes; a
daemonizing command blocks it indefinitely.

**Cause** (`coding/extension/exec.go`, unchanged on main):

- The child is started with `Setpgid: true` (`exec_unix.go:10`), and the
  comment says this is "so a timeout/cancel can target the whole tree", but
  `cmd.Cancel` calls `cmd.Process.Kill()`, which signals the leader PID only.
- The doc comment promises "SIGTERM followed by SIGKILL after 5 seconds";
  the implementation sends SIGKILL immediately.
- No `cmd.WaitDelay` is set, so `cmd.Wait()` waits for every holder of the
  output pipes.

**Suggested fix:** in `Cancel`, signal the group
(`syscall.Kill(-pid, SIGTERM)`, then SIGKILL after the grace period) and set
`WaitDelay` so a stray pipe holder cannot block the call. Windows needs the
equivalent job-object or tree kill.

## 2. PiG: Go SDK cannot see the host's resolved terminal capabilities

**Repro:** [`upstream-repros/pig-go-sdk-terminal-capabilities`](upstream-repros/pig-go-sdk-terminal-capabilities/)
(interactive PTY, settings `{"terminal": {"hyperlinks": false, "images": false}}`,
environment `TERM=xterm-kitty TERM_PROGRAM=kitty`)

```go
caps := tui.GetCapabilities() // the only public source available to a Go extension
ctx.Notify(fmt.Sprintf("extension sees hyperlinks=%t images=%q", caps.Hyperlinks, caps.Images), "info")
```

**Expected:** a Go extension can render with the host's resolved
capabilities: hyperlinks off, images off.

**Actual:** `extension sees hyperlinks=true images="kitty"`. Any Go renderer
built on PiG's public `tui` package (Markdown links, `tui.Image`) emits OSC 8
links and Kitty graphics that the user disabled.

**Cause:** the host sends `terminalCapabilities` in the extension state
snapshot (`coding/extension/host/subprocess/protocol.go:467-471`), and the
Node runtime applies it (`runtime-node/runtime.mjs:2067`). The Go SDK's
`state_update` decoder keeps only `hasUI`, `model`, `session` and `theme`
(`extensions/sdk/extension.go:1161-1168`), and `sdk.Context` has no accessor.
Unchanged on main.

**Suggested fix:** retain the payload and expose it, e.g.
`Context.TerminalCapabilities()`. Ideally also let `tui` components take
capabilities per instance instead of reading the process-global
`GetCapabilities()`: setting that global from an extension is unsafe in packed
cells and in fused binaries.

## 3. PiG: Go SDK overlays are not re-rendered on a height-only resize

**Repro:** [`upstream-repros/pig-go-sdk-overlay-height-resize`](upstream-repros/pig-go-sdk-overlay-height-resize/)
(interactive PTY; the component renders `frame=<n> width=<w> height=<ctx.Height()>`)

**Expected:** after a 30→20 row resize the component renders again with
`height=20`, as it does after a width change.

**Actual:** the host repaints the cached frame `(1, 100, 30)` on the 20-row
terminal; `Render` is not called. A later 100→90 column change produces
`(2, 90, 20)`. Height-dependent layouts (scroll viewports, "80% of the
terminal" browsers) stay wrong until the next key or width change.

**Cause:** `extensions/sdk/extension.go` handles `width_change` by updating
the width and calling `overlay.requestRender()` for every open overlay
(`:1228-1248`). `height_change` (`:1249-1259`) only stores the height.
Unchanged on main.

**Suggested fix:** call `requestRender()` for open overlays on `height_change`
too, and refresh header/footer/widget surfaces as `width_change` does.

## 4. PiG: Piglet JSON Schema rejects `build.extensionRealization`

**Repro:** [`upstream-repros/pig-piglet-schema-extension-realization`](upstream-repros/pig-piglet-schema-extension-realization/)

```yaml
name: probe
build:
  extensionRealization: fused
```

**Expected:** the schema printed by `pig piglet schema` accepts every field
that `pig piglet validate` accepts, since `piglets.md` documents the field
and says schema, validate, show and build "all use the same closed source
contract".

**Actual:** `validate` accepts `fused` and rejects other values (so the field
is implemented), but `$defs.buildSpec` lists only `targets` and `outputName`
with `additionalProperties: false`. A schema-validating editor or CI check
rejects a valid Piglet. Source: `coding/piglet/piglet.schema.json`
vs `coding/piglet/types.go:347,752`. Unchanged on main.

**Suggested fix:** add `"extensionRealization": {"enum": ["fused"]}` to
`buildSpec`, and a test that every field accepted by the validator is in the
schema.

## 5. PiG: non-git `PIG_SOURCE_ROOT` fails with `exit status 128`

**Repro:** [`upstream-repros/pig-source-root-not-git`](upstream-repros/pig-source-root-not-git/)
(`git archive` export of the matching commit as `PIG_SOURCE_ROOT`, one fused
extension)

**Actual:** `builder native failed after execution started; no fallback was
attempted: resolve Pig source revision: exit status 128`.

**Expected:** an actionable message such as "PIG_SOURCE_ROOT must be a git
checkout (`git rev-parse HEAD` failed: not a git repository)", or support for
plain source trees that carry the version some other way. The troubleshooting
docs say only that a source checkout is needed.

**Cause:** the revision lookup runs `git rev-parse HEAD` and wraps only the
exit status, dropping git's stderr (0.3.1 `coding/pigletbuild/records.go:567`;
main `coding/pigletbuild/source_fetch.go:322-334`). Release-fetched sources
avoid this on main; local exports still hit it.

## 6. PiG 0.3.1: extension `ctx.Compact()` is a silent no-op in RPC/print mode — fixed on main

**Repro:** [`upstream-repros/pig-rpc-extension-compact-noop`](upstream-repros/pig-rpc-extension-compact-noop/)
(RPC mode, `compaction.keepRecentTokens=1`, three prior turns)

**Actual (0.3.1):** `ctx.Compact(nil)` returns and no `session_before_compact`
follows within 5 s. `CompactWithOptions{OnError}` reports `compaction is not
available`. The RPC client's own `compact` command succeeds on the same
session.

**Cause:** RPC and print modes register no `compact` host action
(`cmd/pig/session_extension_actions.go`), and
`UIBridge.handleCompact` returns success when the action is missing and no
completion was requested (`ui_bridge.go:1783-1788`). Main registers the
action (`session_extension_actions.go:278`). Remaining suggestion: make the
missing-action path an error rather than a silent success.

## 7. PiG 0.3.1: no public highlighter that takes a theme — fixed on main

In 0.3.1 the only public highlighter, `tui.HighlightCode(code, lang)`, reads
the process-global `ActiveTheme()`. In an extension process that is not the
host's theme, and mutating it would affect other packed or fused extensions.
So a Go extension rendering Markdown with the host theme
(`tui.NewMarkdownWithOptions` + `MarkdownTheme` from `ctx.UITheme()`) cannot
highlight code in the host's syntax colors. In-repo red proof:
`test/fixtures/pins-render-probe` + `python3 -m test.integration.pins_render_pty`
(a custom theme's `syntaxKeyword` is ignored). Main adds
`tui.Highlight(code, HighlightOptions{Theme: …})`, which allows a per-instance
theme. This repo's pins extension works around 0.3.1 with a local copy of the
lexer-to-theme mapping.

---

## 8. pi-schedule v0.4.0: queued scheduled turns run with a later job's privilege tier

Upstream: `pungggi/pi-schedule` `ed5ea93f1a82ac7038fda34acbbfaa572472f9e3`.
Repro (vitest, **not executed**):
[`upstream-repros/third-party/pi-schedule/queued-tier.test.ts`](upstream-repros/third-party/pi-schedule/queued-tier.test.ts)

When two jobs are due in one wave, the runner sends job A, calls
`privilege.enter(A.tier)`, then sends job B as a follow-up and calls
`privilege.enter(B.tier)` (`src/runner.ts:304-309,726`), before either
agent turn has run. `tool_call` checks the **top** of the stack
(`src/privilege.ts:111`), and one `agent_settled` pops one level after the
whole run (`:195`). With A=`read_only` and B=`mutate`, A's turn can run
`bash`, `edit` and `write`: a privilege escalation of the read_only job.

Why a stack cannot work: in one run, a queued follow-up gets `message_start`
but no `before_agent_start`, and the run settles once
([`upstream-repros/pig-followup-event-order`](upstream-repros/pig-followup-event-order/),
observed on PiG; Pi's single-prompt runs suggest the same, but that was not
run here). This repo's Go port binds each tool call to the latest user
message on the active branch. Its real-host test
(`python3 -m test.integration.schedule_rpc`, `queued_tiers`) shows A's `bash`
blocked and B's allowed in the same run.

**Suggested fix:** associate each reserved prompt text with its tier and, in
`tool_call`, look up the tier of the user message that owns the current turn
(e.g. the latest user entry on the branch) instead of a stack top.

## 9. pi-schedule v0.4.0: stale run completion resurrects cancelled jobs and undoes disables

Repro (vitest, **not executed**):
[`upstream-repros/third-party/pi-schedule/stale-attempt.test.ts`](upstream-repros/third-party/pi-schedule/stale-attempt.test.ts)

`markAttempt()` builds the updated row from the runner's copy taken before
delivery (`src/store.ts:487-505`) and `upsert()` re-appends a row that is no
longer in the file (`:469`). If the user cancels a job while its shell
command or delivery is in flight, the job reappears. If they disable it, it
is re-enabled. **Suggested fix:** inside the file lock, apply the attempt to
the freshly read row, and do nothing if it is gone.

## 10. pi-schedule v0.4.0: skill promises "no prior conversation"

`skills/schedule/SKILL.md:20-21` says each prompt job is "an isolated turn — a
brand-new user message with no prior conversation", and the injected contract
says "This is an isolated scheduled run". The runner delivers with
`pi.sendUserMessage` into the **current** session (`src/runner.ts:390,394`),
so the full history is in context. The advice to write self-contained prompts
is right, but the isolation claim is not.

## 11. pi-imgview: missing browser opener gives false success and an unhandled spawn error

Upstream: `gregjohnso/pi-imgview` `17b568e8e3b70d009adb8ac090d2b3280f065f11`.
Repro (node:test, **not executed**):
[`upstream-repros/third-party/pi-imgview/missing-opener.test.ts`](upstream-repros/third-party/pi-imgview/missing-opener.test.ts)

`openInBrowser()` spawns `xdg-open`/`open` detached and returns immediately
(`extensions/imgview/utils.ts:295-299`), without an `error` listener. With no
opener on PATH (headless servers, containers, minimal VMs) `spawn` reports
ENOENT asynchronously. The `try/catch` in `index.ts:160-161` never sees it,
the tool result says `Browser: opened <path>.`, and the child's `error`
event, having no listener, is raised as an uncaught exception in the host
process. **Suggested fix:** await the `spawn` or `error` event before
reporting, and attach an `error` listener.

## 12. pi-pins: `/pin list` documented but missing; `/pin help` opens the browser

Upstream: `s4lv0/pi-pins` `776217ccdce52aa0ae7794998847ff2253677250`.

- The header documents `/pin list` (`extensions/pin.ts:10`), but `list` is
  not a subcommand, so `/pin list` pins the last answer with the label "list".
  The source deliberately keeps `/pin list of plugins` as a label (`:450`), so
  a fix should accept `list` only alone or with a pin number.
- `/pin help` opens the pin browser instead of showing help whenever pins
  exist (`:382-384`).

Manual repro: pin one answer, then run `/pin list` (it creates pin #2 labelled
"list") and `/pin help` (it opens the browser).

---

## Checked and not filed

- **Interactive input during extension startup:** PiG holds submissions with
  "Startup is still in progress" and keeps the text in the editor. Not a bug.
- **Extension `sendUserMessage` during compaction (interactive):**
  `upstream-repros/pig-send-during-compaction` shows PiG returns no error and
  delivers the message after compaction finishes. That is better than
  pi-schedule's assumption that Pi throws, and not a bug.
- **Queued follow-up prompts get no `before_agent_start`**, and a run settles
  once (`upstream-repros/pig-followup-event-order`), consistent with one
  prompt per agent run. Extension authors should not use
  `before_agent_start` to track per-message ownership (see issue 8).
