# PiG extension port handoff

## Current continuation status

The user authorized **atomic commits**. Continue the full plan; Goal-mode state and
tool authority come only from the latest live conversation contract, never from
this handoff. The current objective includes Codex and the unfinished extensions.

Implemented and committed:

- `59b0431` — initial plan/handoff, project rules and licensing/provenance.
- `94254a9` — savelast factory plus pure, SDK read-error, real RPC and compiling
  whole-log mutation tests.
- `65d3f87` — independent notify-pushover factory, offline HTTP tests, real command
  and model/tool dispatch, abort/disconnect teardown, notices and user docs.
- `361bfb2` — savelast ECMAScript whitespace fix: BOM-only warns, NEL is saved.
- `d8e652c` — Codex factory, parsing/presentation, HTTP/reset and lifetime owner,
  unit/race tests and real adapter/settlement command evidence.
- `1f57fc5` — concrete scheduler metadata approval gate and rendering probe notes.
- `5991fd9` — real native OAuth/WHAM caller and model/session/disconnect proofs;
  shared private TLS fixture, isolation hardening and buffered shutdown evidence.

Authoritative current evidence is `docs/validation.md`; source notices are in
`docs/provenance.md`. All three modules pass Go 1.27.1 unit/race/vet checks. The default
Go now reports 1.26.8; use `GOTOOLCHAIN=go1.27.1` for the pinned cached toolchain.
Python probes are packages: run `python3 -m test.integration.<probe>`, not file
paths. New helper discovery required `pyrightconfig.json`; fresh Python LSP checks
are clean. The RPC event waiter supports a `since` marker because settlement can
arrive before a command response. Do not wait for a duplicate `agent_end`.

The RPC driver now has `send`/`wait_response` for held operations and drains final
stdout frames on close. It forces `PIG_USE_PI_DIRS=0`, both agent-directory overrides
and both offline flags, so ambient shared-mode settings cannot select live credentials.

No real account/provider/WHAM requests or credit redemption were performed by tests;
no installation, remote or PiG core change was made. A harness Pushover alert did ask
the operator for scheduler host-scope approval; it was not an extension delivery test.
PiG remains at the target commit with a clean tree. Interactive reload and packed
placement are still combined-qualification gates; RPC shutdown is not terminal proof.

Next: pins/imgview rendering feasibility and implementation, then scheduler pure
logic and runtime only if its startup-metadata gate is resolved. Codex native OAuth,
all reset acknowledgements, UUID/body/claim/header correctness, model/session stale
suppression, HTTP disconnect and new-epoch usability now have real PiG caller proofs.
Run `python3 -m test.integration.codex_usage`, `codex_native` and `codex_lifetime`.

Pins source remains at `../kmet/target/reference/pi-pins-source/extensions/pin.ts`,
commit `776217ccdce52aa0ae7794998847ff2253677250`, SHA-256
`f32d9f61508d882ffe4e780095f5c83cda391d4f6526fa364b19523f9e6e1262`.
Public `tui.NewMarkdownWithOptions` accepts explicit `MarkdownTheme` callbacks,
but `tui.HighlightCode` reads process-global `ActiveTheme`; Markdown link rendering
also reads global capabilities. Probe per-factory theme/capability handling rather
than mutating shared globals or writing a competing renderer.

Pinned original Codex source and license are still at
`/tmp/pig-extensions-review.JPGJpX/{pi-codex-usage.ts,codex-LICENSE}`; their hashes
match the plan. This temporary location may disappear; use the pinned origin and
verify digests rather than treating the path as permanent project provenance.
Scheduler startup metadata remains unresolved; source proof/design is in
`docs/feasibility.md`. Operator approval for separate host metadata work (or explicit
runtime deferral) was requested while independent work continues. Do not silently
omit suppression or edit PiG without approval. `plan.md` had pre-existing formatting
changes at this continuation's start and was deliberately left unstaged.
No BTW or extension-toggle implementation has been started.

## Historical initial snapshot

The remainder records the state before the continuation above. Its no-commit,
untracked-file and missing-test claims are historical, not current instructions
or acceptance evidence.

This document records the original implementation state so work can continue from `../pig-extensions` without reconstructing the earlier review.

## Project identity and repository state

- Project path: `/home/user/dev/jail/pig-extensions`.
- Run project commands from this directory, or use the explicit paths below.
- Git was initialized with initial branch `main`.
- The repository has no commits and no remote.
- Current project status is intentionally uncommitted:
  - `plan.md` is untracked.
  - `pi-handoff.md` is untracked.
  - `extensions/savelast/` is untracked.
- Do not create a remote, commit, install extensions globally, or publish without a separate request.
- Repository/module namespace: `github.com/jasalt/pig-extensions`.
- Copyright holder for new work: Jarkko Saltiola.
- New project work is MIT. Preserve every third-party license and attribution instead of replacing it with the project license.

The sibling PiG checkout is `/home/user/dev/jail/PiG` at commit `0e6ed0048282a15531ea1652a5833de4da4dd1a7`. Its working tree was clean after the build. The qualified build toolchain is Go `1.27.1`; the module language floor is Go `1.26`. Node is not on `PATH`, and the selected production design is Go-only.

## User-ratified scope

1. Prioritize the six candidates with source: `savelast`, `nofity-pushover` ported as `notify-pushover`, `codex-usage`, `pins`, `imgview`, and `schedule`.
2. Prefer Go for every extension. Do not introduce a Node runtime dependency unless a later decision changes this.
3. Use the original Pi extensions as the main behavioral reference. Use kmet as a secondary implementation and regression reference. The kmet `pins` presentation is the preferred visual style.
4. Target the current PiG checkout, not a published binary. Do not change PiG core without separate approval.
5. Use independent Go factories, each selected from an exact extension root. Do not make an umbrella Package or Piglet a prerequisite.
6. Qualify Linux first. Do not claim other platforms, desktop/browser bridges, terminals, or fused Binary delivery without evidence.
7. `savelast` reads the latest assistant response on the active branch. It must not select an abandoned branch.
8. Codex usage uses native PiG notifications and retains immediate reset redemption. Never redeem a real account in tests.
9. The extension directory is `notify-pushover`. Keep tool `notify_human` and command `/notify-human-test`. Use PiG-specific `PIG_PUSHOVER_USER_KEY`, `PIG_PUSHOVER_APP_TOKEN`, `PIG_PUSHOVER_DEVICE`, and a PiG agent-directory-scoped JSON file. Do not accept legacy environment aliases or migrate old credentials.
10. `imgview` uses native PiG image capabilities and retains its warning-only image threshold and browser-viewer behavior, subject to real transport limits.
11. `schedule` follows original pi-schedule v0.4.0 scope. Its implementation injects prompts into the current Session; it does not create a history-free child Session. Preserve all original kinds, actions, trust behavior, privilege policy, relaxed options, and reliability controls.
12. Preserve scheduler policy, including project trust, explicit `run_now` bypass, strict read-only default, `suggest`, and opt-in legacy mode. Do not add a confirmation gate without approval.
13. BTW is conditional only after the six source-bearing ports. Target original pi-btw v0.7.1, prefer Go, and do not implement child-extension allowlists or ambient child extension loading. Defer it if child-session/provider/UI substrate is not practical.
14. Do not implement `extension-toggle`; document and use `pig config` and `pig config --local`.
15. Use fresh PiG JSON/JSONL state with no kmet import or compatibility reader. The local notifier source is authorized for adaptation. Retain kmet's 15-second notifier timeout and redirect refusal.

The full decision record, source provenance, feasibility gates, milestone sequence, and acceptance matrix are in `plan.md`.

## Current implementation: `extensions/savelast`

Files:

- `extensions/savelast/extension.go`
- `extensions/savelast/core_test.go`
- `extensions/savelast/go.mod`
- `extensions/savelast/go.sum`
- `extensions/savelast/README.md`

Module identity:

```text
github.com/jasalt/pig-extensions/extensions/savelast
```

SDK dependency:

```text
github.com/MichaelKinsy/PiG/extensions/sdk v0.3.1
```

Factory:

```go
func Extension() *sdk.Extension
```

Registered capability:

```text
/savelast [path]
```

The factory does the following:

- Calls `ctx.SessionManager().GetBranch(nil)` for the active branch.
- Walks entries backward and selects the latest nested assistant message.
- Treats a latest assistant message with no usable text as textless; it warns and never falls back to an older response.
- Preserves string content exactly.
- For array content, joins only blocks whose `type` is `text` and whose `text` is a string, using a single newline between accepted blocks. Thinking, tool-call, image, malformed, nil, and non-string blocks are skipped.
- Resolves a nonblank argument against `ctx.Cwd()` with normalized paths.
- Uses `<Unix milliseconds>.md` under `ctx.Cwd()` when the argument is blank.
- Creates parent directories, writes UTF-8 bytes without adding a final newline, and overwrites an existing file without confirmation.
- Emits PiG notifications for no assistant, textless assistant, read failure, write failure, and success. It returns nil after reporting command failures, matching the original command’s user-facing error handling rather than producing a second generic command error.
- Performs no network operation, browser launch, scheduler start, credential read, or account mutation during construction or registration.

The implementation is adapted from `atomdmac/pi-savelast` at commit `efb580c1e7f95e230c2c413021b6680abfa287c7`. Its package metadata declares ISC, not MIT. The original reference checkout is `../kmet/target/reference/pi-savelast`; preserve that third-party licensing/attribution in the eventual provenance/license files. Do not assume its `GetEntries()` behavior for PiG: the user explicitly selected active-branch behavior.

## Evidence completed

From the PiG checkout:

```sh
go build -o bin/pig ./cmd/pig
./bin/pig --version
# 0.3.1+0.87.1
```

From the extension module:

```sh
cd ../pig-extensions/extensions/savelast
go test ./...
go test -race ./...
go vet ./...
```

All three commands pass after the implementation was added.

Validation with an isolated PiG home:

```sh
cd ../PiG
home=$(mktemp -d)
PIG_HOME="$home" ./bin/pig install --validate-only --json ../pig-extensions/extensions/savelast
```

The result was `valid: true`, `registered: true`, factory `Extension`, language `go`, package `github.com/jasalt/pig-extensions/extensions/savelast`, and runtime `subprocess`/`packable: true`. The reported source hash was `54b1e83f28b70a7e0648e88646c91dc1eb6cc88c4cf356ae37ac9fbf95ea8820`.

A real RPC host probe also passed. It used a temporary Session file with an abandoned assistant branch and a different active assistant branch. The driver waited for the `get_commands` response before sending the extension command, then waited for the `/savelast` response before closing stdin. The command wrote exactly `ACTIVE RESPONSE`, not `ABANDONED RESPONSE`, and emitted:

```text
Saved to: <temporary-project>/output.md
```

The command protocol is asynchronous. A single shell pipe that sends a command and immediately closes stdin can close RPC before the extension command is admitted. Do not treat that race as a product failure; use a driver that waits for command admission/response.

The first attempted RPC probe used unavailable model `test-faux/echo` and failed before extension execution with `Model "test-faux/echo" not found`. Offline `--list-models` also reported no configured models in a fresh home. Extension commands can be tested without selecting a model by using RPC mode with no `--model`, as in the successful probe.

## Test coverage currently present

`core_test.go` covers:

- latest assistant selection;
- only valid text blocks;
- preservation of exact string content, Unicode, tabs, CRLF, and trailing spaces;
- no fallback when the latest assistant is textless;
- absent assistant messages;
- relative and absolute path normalization;
- epoch-millisecond default names;
- parent directory creation;
- no added newline;
- overwrite behavior.

Still required for the complete savelast acceptance matrix:

- command-level no-assistant and textless notifications through real RPC;
- command-level read/subscription failure notification;
- mkdir/write failures through real RPC;
- blank/default arguments through real RPC;
- paths with spaces and Unicode through real RPC;
- fresh Session/replacement behavior;
- poisoned assistant/error/aborted messages and exact source behavior for those shapes;
- a compiling mutation proving the active-branch scenario fails if `GetEntries()` or an older assistant fallback is substituted;
- final source/provenance notice and license inventory.

Do not weaken comparisons or turn the host test into registration-only evidence.

## Important environment/test notes

- `go test ./...` was accidentally run from the PiG root while setting up the task. It is not a valid extension test. It hit pre-existing environment/toolchain limitations: missing `extensions/sdk-ts/node_modules`, missing Node differential probes, and a temporary disk-quota failure during the broad build. Do not rerun the full PiG suite for this extension until the required toolchain and Node dependencies are approved/available.
- PiG’s source working tree remained clean. The build output `bin/pig` is ignored or otherwise not a tracked change.
- Do not touch the modified/untracked files in `../kmet-extensions`; the kmet checkout changed after the initial review and is not the authoritative source.
- Use temporary homes, session files, project directories, and stores. Do not write into checked-in fixture roots.

## Recommended next steps

1. Add the root MIT `LICENSE` naming Jarkko Saltiola, plus a provenance/third-party notice file that preserves the ISC savelast metadata and all later upstream notices. Do not incorrectly relicense third-party code.
2. Add a real RPC test driver or a PiG-side integration test fixture for `savelast` boundary cases. Keep the driver synchronized by waiting for `get_commands` and the command response.
3. Complete savelast’s command/error matrix and run the active-branch mutation proof.
4. Port `notify-pushover` as an independent module at `extensions/notify-pushover`. Preserve the local Pi tool/command contract, switch to `PIG_PUSHOVER_*`, use the selected PiG agent directory, and retain a 15-second HTTP timeout plus redirect refusal. Keep tests offline with a local HTTP server/client seam; never send a real alert during normal tests.
5. Port `codex-usage`, then qualify Go rendering feasibility before `pins` and `imgview`. Keep native status/notification and image behavior explicit.
6. Port scheduler pure logic before runner/lifecycle. First resolve the supported parent-host initial-prompt metadata path; do not use `/proc`, infer from empty history, or silently omit startup suppression.
7. Defer BTW until a written Go child-session/provider/UI feasibility design passes review. Do not implement `extension-toggle`.

## Useful commands from this directory

```sh
cd /home/user/dev/jail/pig-extensions/extensions/savelast
go test ./...
go test -race ./...
go vet ./...

cd /home/user/dev/jail/PiG
go build -o bin/pig ./cmd/pig
home=$(mktemp -d)
PIG_HOME="$home" ./bin/pig install --validate-only --json ../pig-extensions/extensions/savelast
```

The canonical plan is `plan.md`. This handoff is an implementation snapshot, not a replacement for that plan or for the PiG repository instructions.
