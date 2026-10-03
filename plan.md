# pig-extensions project plan

Status: **Initial `savelast` Go factory implemented and host-validated. Continue with remaining source-bearing extensions.**

This workspace contains this plan, an uncommitted local Git repository initialized on `main`, and the initial `extensions/savelast` Go module. It has no commits or remote. No extension is installed globally and no credentials or live external effects were used.

## 1. Goal and boundaries

Port the applicable extensions from `../kmet-extensions/` into independent Go-first PiG extensions. Use the original Pi extensions as the main behavioral reference. Use kmet as a secondary implementation/test reference, and use its `pins` presentation as the visual reference. Record approved behavior changes rather than treating every kmet adaptation as authoritative.

PiG is a Go implementation of [Pi](https://github.com/earendil-works/pi), the reference implementation for its host behavior. [Pi's extension documentation](https://pi.dev/docs/latest/extensions) describes that reference contract. Michael Kinsy created PiG, which was originally developed at Hewlett Packard Enterprise. These facts do not imply endorsement of this extension collection.

All candidates are optional workflow capabilities. They belong in this separate project, not in Stock PiG or its default loadout. Select each extension by its exact source root. A Package is an optional distribution envelope, not a requirement for a Go factory. This project does not automatically modify PiG Standard, Home Manager, live settings, or existing kmet installations.

Selected direction:

- Prioritize the six source-bearing candidates: `savelast`, `notify-pushover`, `codex-usage`, `pins`, `imgview`, and `schedule`. Implement the five smaller utilities before qualifying the scheduler.
- Prefer independent Go factory modules. Do not add a Node runtime dependency or a mandatory umbrella Package.
- Consider Go `btw` at the reviewed original v0.7.1 only if PiG offers a practical route simpler than kmet's missing child-session substrate. Exclude `extension-toggle`; `pig config` is sufficient.
- Target this PiG checkout and Linux. Qualify the actual Linux terminal/browser path; do not promise other platforms or fused Binary delivery.
- Keep PiG core unchanged unless a concrete host change receives separate approval.
- Keep every extension independently selectable. Do not activate the scheduler or notifier merely to test other extensions.
- Use fresh PiG JSON/JSONL state and PiG-specific configuration. Do not import kmet sessions, schedules, trust grants, or credentials.
- License new project work under MIT. Preserve third-party licenses and notices, including savelast's ISC terms. The user authorizes adapting the local notifier source.
- Do not substitute a one-shot completion for BTW's child agent, prompt wording for scheduler enforcement, or registration-only tests for working features.

## 2. Review baseline and evidence limits

| Item | Reviewed baseline |
|---|---|
| PiG checkout | `../PiG`, commit `0e6ed0048282a15531ea1652a5833de4da4dd1a7`; working tree clean during review |
| PiG version pin | `0.3.1`, targeting Pi `0.87.1`, upstream commit `f07218c4d4bbc12bef056a7058c3dd49dfe41abe` |
| Installed binary | `/home/user.guest/go/bin/pig`; `pig --version` returned `0.3.1+0.87.1` |
| kmet extension checkout at initial review | `../kmet-extensions`, HEAD `13cdeea244679efa88f86f9c31836c165731d354` |
| Source worktree qualification | The initial worktree contained modified `codex-usage/README.md` and `codex-usage/scripts/smoke.bb`, an untracked root README, and untracked demo images. The review used that current source/docs and left all existing changes untouched. HEAD alone does not identify that complete worktree. |
| Later kmet checkout observation | While recording the decisions, HEAD advanced to `3ff769d4aa07857f31c105b8873dbac2762aa454`, with `README.md` modified. This review made no kmet changes and does not certify that later snapshot. Recheck any kmet material used during implementation. |
| Available native toolchain | `go version` returned `go1.27.1 linux/amd64` |
| Optional Node oracle prerequisite | `node --version` failed during initial review: command not found. No tool was installed. The selected Go production design does not require Node; executing original TypeScript oracle tests may require it. |
| Environment | Linux x86-64 in Lima. The Fedora/Wayland desktop is outside the VM; browser launching must be qualified across that boundary. |

This is a source-and-documentation review, not a runtime acceptance run. The review read all candidate READMEs and both port designs, the implemented extension entry points, scheduler lifecycle and privilege code, pin model/UI code, image-loading code, and representative regression tests. It also inspected PiG's SDK, wire types, production notification path, Node SDK exports, and existing test/evidence sources.

At the initial source review, no extension was loaded. No live provider request, Pushover alert, reset redemption, browser launch, schedule, or settings mutation was performed. No test suite or PTY replay had been run. Existing PiG tests cited below were available evidence to rerun, not tests newly passed in that review.

After the user's decisions, the review retrieved original pi-schedule v0.4.0 and the pinned Codex source/license into a temporary directory. It inspected the scheduler's runner, privilege enforcement, initial-prompt detection and tests, README, reliability notes, and skill. It also inspected the original local notifier, savelast implementation, and BTW child-extension selection. No downloaded source was executed.

Current implementation evidence: `extensions/savelast` now has a Go factory, unit tests, README, module files, and no external runtime dependency beyond the pinned PiG SDK. `go test ./...`, `go test -race ./...`, and `go vet ./...` pass in that module. A PiG binary built from checkout `0e6ed0048282a15531ea1652a5833de4da4dd1a7` reports `0.3.1+0.87.1`; `install --validate-only --json` registers `savelast` successfully. A real RPC command against a seeded branched Session wrote the active-branch assistant text, omitted thinking, and emitted the expected notification. This does not close the full savelast matrix or the remaining extensions.

Some materialized PiG documentation is behind the checkout: examples include Go getter signatures and claims that Node factories always remain isolated. The checkout's current SDK, runtime-cell documentation, parity matrix, and production source govern implementation. Equal release strings do not prove that an installed binary contains every reviewed checkout change.

### Source provenance

| Candidate | Origin recorded or inspected |
|---|---|
| `savelast` | `atomdmac/pi-savelast`, `efb580c1e7f95e230c2c413021b6680abfa287c7`; local reference manifest declares ISC, not MIT |
| `notify-pushover` | Local Pi `pushover-human` at `../lima-default/home-manager/pi/agent/extensions/pushover-human/`; user authorizes adapting this source. The kmet directory remains named `nofity-pushover`; do not rename it in place. |
| `codex-usage` | `jasalt/chatgpt-openai-api-adapter/contrib/pi-codex-usage.ts`, `94f45568b4bd7842b1aef362cc3ba883b1312951`; MIT license retrieved at that commit, copyright 2026 Jarkko Saltiola |
| `pins` | `s4lv0/pi-pins`; local reference checkout HEAD `776217ccdce52aa0ae7794998847ff2253677250`; retained MIT license; kmet supplies the selected presentation |
| `imgview` | `gregjohnso/pi-imgview`, `17b568e8e3b70d009adb8ac090d2b3280f065f11`; retained MIT license |
| `schedule` | `pungggi/pi-schedule` tag v0.4.0 resolves to `ed5ea93f1a82ac7038fda34acbbfaa572472f9e3`; MIT license inspected, copyright 2025 Alessandro Pungitore |
| `btw` | Conditional original v0.7.1 target: `dbachelder/pi-btw`, `d0d1ba5404b66058c501ed3e286733660df56aa9`; clean local reference checkout and MIT license exist; child-extension loading is excluded |
| `extension-toggle` | Not ported. Use PiG's existing `pig config` / `pig config --local`. |

Additional source-review SHA-256 records:

| Material | SHA-256 |
|---|---|
| pi-schedule archive from `https://codeload.github.com/pungggi/pi-schedule/tar.gz/ed5ea93f1a82ac7038fda34acbbfaa572472f9e3` | `e7ec9ec3120a99770834a857c65dde8ad62fcf2be6dfc4efb3c95791c2b4dfc4` |
| pi-schedule `src/runner.ts` | `d14a07bbd52c8505cbe5f1e97243d708037a79565b5dae69c43679bdd0140e22` |
| pi-schedule `src/privilege.ts` | `296f95663ef339f25832f88d9f467fdf278f6887833a104da22eea66030b5a1d` |
| pi-schedule `src/cli-prompt.ts` | `014bcaa247da75246b1213bffd4b02e6d018911f1a9de0268403e758f742987e` |
| pi-schedule `LICENSE` | `5a4c2f72c4d2acfe7833b8160a339662f31c0930915b9b44009d42f24bdede49` |
| Pinned `contrib/pi-codex-usage.ts` | `9e6bf72c5e050b51d7825a667a351062afe6b81bb737507a9f4175e181cea3b7` |
| Codex repository `LICENSE` at the pinned commit | `b4e8cd39baa974c8d693b5f8c7078ace3a481172f539e0e42a57075d675911e7` |
| Local Pi `pushover-human/index.ts` | `28fdb9c1914c67b08c89fc0cb0c4e06d657db90f7a403f340394f71a5a991a1f` |

Record exact origin, version or commit, content digest, license, and intentional behavior changes per extension. Do not claim license clearance for the entire collection from the licenses of a few members. Do not copy ignored `target/` artifacts or demo assets without a reason and provenance.

## 3. PiG capability assessment

### 3.1 Supported mechanisms relevant to the ports

| Need | Current PiG mechanism | Implication |
|---|---|---|
| Extension loading | Go `Extension() *sdk.Extension`, Rust/Python factories, Pi-compatible Node default exports, or exact standalone programs | No Clojure loader or `extension.edn` translation is needed. Use a supported SDK, not a new wire implementation. |
| Independent selection | Exact extension roots, Package resource inventory, `-e`, settings, Piglets | A repository containing several factories is not itself one unambiguous extension. Select exact members. |
| Commands/tools/events | `Command`/`RegisterCommand`, `Tool`/`RegisterTool`, `OnEvent`, shortcuts, argument completions, tool updates | Covers the basic commands and tools in the six source-bearing candidates. |
| Session reads | `Context.SessionManager()` exposes raw entries, active branch, session identity, and context building | Both active-branch and full-log behavior are possible. Use raw message content when whitespace/block boundaries matter. |
| Durable extension state | `AppendEntry` and session branch reads | Pin snapshots and BTW thread metadata can remain outside model context. Do not parse live session files independently. |
| Message delivery | `SendMessage` and `SendUserMessage`, with explicit delivery and optional trigger flags | Use explicit `TriggerTurn: sdk.Bool(false)` for non-triggering messages. A custom message is persisted and can affect future model context; it is not a transient notification. |
| Status and notifications | Keyed `SetStatus`; `Notify`; cached widgets/renderers | Do not replace the entire footer for quota or notifier status. Production `Notify` writes chat status/error UI; successive info statuses can replace the last status. It is not equivalent to kmet's append-only `ui-chat-info`. |
| Focused UI | `Custom`, `RemoteComponent`, layout options, terminal dimensions, invalidation and disposal | A single pin browser is plausible. Geometry, render width, focus, cancellation, and concurrent overlay interaction still require real-host proof. |
| Model/auth access | Current model metadata, `GetModelAuth`, `ModelRegistry` facade and host model streaming | Codex usage can use resolved credentials and headers without reading auth files. A model stream alone is not a tool-running child agent. |
| Tool permissions | `tool_call` blocking, `before_agent_start`, active-tool APIs, settlement events | Scheduler policy can be enforced through hooks, but exact queued-run ownership and tool surfaces must be tested. An extension is not an OS sandbox. |
| Images | Ordered text/image tool-result content; host image normalization; custom message renderers | Translate kmet's separate `:images` field into PiG's ordered content blocks. Slash-command images need a separately qualified renderer. |
| Files, HTTP and processes | Ordinary language facilities; host `Exec` with cancellation; PiG configuration paths | Reuse the appropriate public boundary. Own HTTP cancellation, proxy behavior, response-body cleanup, filesystem errors, and process lifetimes. |
| Skills | Package skills or `resources_discover` | Distribute the scheduler skill as a Resource. Do not invent runtime skill registration equivalent to kmet's API. |
| Execution placement | Isolated and packed factories; compatible Go factories can fuse into Piglet Binaries | Keep state per factory. Never depend on package globals, process-wide cwd changes, direct terminal writes, or other members of a packed cell. |

The Go module floor is 1.26; the qualified build toolchain is Go 1.27.1. Use `github.com/MichaelKinsy/PiG/extensions/sdk`, not the legacy module path. Pin the agreed SDK and let PiG stage it for source builds. Do not commit machine-local `replace` paths.

### 3.2 Important limits and non-equivalences

1. **BTW is not blocked by the same missing API as kmet.** The Node compatibility runtime exports real independent `createAgentSession`, `ModelRuntime`, resource-loader and session-manager implementations. `runtime_node_child_session_test.go`, `runtime_node_child_lifetime_test.go`, and `test/parity/unit-evidence/replay-node-btw.py` provide relevant tests. Existing replay references include pi-btw 0.6.1; they do not certify the selected 0.7.1 feature set. These imported Node package exports are not a `CreateAgentSession` method in the multi-language extension SDK. A Go-only BTW requires a separate design over the public Go coding SDK or another approved boundary, including parent-provider/auth/tool inheritance.
2. **Package services are not a reason to build a toggle extension.** The Node SDK exports `DefaultPackageManager` and `SettingsManager`, while the shared extension SDK lacks a reviewed resource snapshot/apply operation. The user selects `pig config`, so no new extension-owned settings service is needed.
3. **kmet's proposed package `enabled` gate is not PiG's current package schema.** PiG's `PackageSource` has `source`, `autoload`, and resource filters, not `enabled`. Do not add that field or reinterpret `autoload`. Use the existing `pig config` resource-filter semantics.
4. **Custom UI is not completely parity-closed.** `docs/extension-api-parity.md` and `docs/findings/fix-overlay-shim.md` record serialized floating calls, missing Node factory `tui.showOverlay`, and remaining input/layout issues. Single-overlay support does not prove BTW focus switching or simultaneous pin/BTW views. Existing D73 also limits live main-process object identity.
5. **Go's extension SDK is not a full Markdown/Image widget library.** It transports rendered lines and image tool results. Qualify reuse of PiG's public rendering packages and theme/capability propagation. Do not write a competing Markdown renderer or terminal image protocol as a shortcut. If Go cannot support the selected behavior, report the blocker before proposing Node or host changes.
6. **Provider and lifecycle gaps remain documented.** D74 limits some direct Node provider options/results; D78 covers remaining Provider-object boundaries; D30/D70 affect retained contexts and session replacement. Test only the provider shapes and lifecycle behavior the selected ports actually require. Do not claim blanket SDK parity.
7. **Background work needs an owner.** Codex polling and scheduling outlive one handler. Give them explicit cancellation, generation checks, error ownership, and shutdown joins. Obtain fresh session context after replacement. A completed request context is not a suitable lifetime for an HTTP poller or scheduler.
8. **Identity and paths change deliberately.** Use PiG's selected configuration root and agent-directory rules, including overrides. Keep extension state namespaced. Do not copy `.kmet` paths or `originator: pi` blindly; PiG's identity policy is D2/D26.

## 4. Applicability and port specifications

“Applicable” below means there is a reasonable implementation path. It does not mean that the extension has been ported or runtime-qualified.

| Extension | kmet state | Selected disposition | Relative risk |
|---|---|---|---|
| `savelast` | Implemented | Initial Go factory and active-branch path implemented; complete boundary matrix remains | Low |
| `notify-pushover` | Implemented as `nofity-pushover` | Second Go port; corrected name and PiG-only configuration | Low–medium |
| `codex-usage` | Implemented | Go port; native notifications and immediate reset redemption | Medium |
| `pins` | Implemented | Go port; original Pi behavior with kmet-style presentation | Medium |
| `imgview` | Implemented | Go port using native PiG image capabilities | Medium–high |
| `schedule` | Work in progress | Go port of original Pi scope and policy; separate reliability milestone | High |
| `btw` | Design only | Conditional Go feasibility review at original v0.7.1; no child-extension loading | High |
| `extension-toggle` | Design only | Excluded; existing `pig config` is sufficient | None added |

### 4.1 savelast

Preserve `/savelast [path]`, default epoch-millisecond `.md` filenames, whole-argument paths with spaces, current command cwd, parent creation, UTF-8, overwrite behavior, and no added final newline. Preserve the latest assistant's text blocks only. A textless latest assistant warns; it must not fall back to an older answer.

Use `SessionManager().GetBranch(nil)` and select the latest assistant response on the active branch. This is the approved difference from original Pi savelast, which uses `GetEntries()` and can include abandoned branches. Preserve the original text extraction and overwrite-without-confirmation behavior. Do not use a convenience projection that changes text whitespace or joins blocks differently.

Acceptance: encode the original source's inputs and expected bytes, using kmet's tests as additional regression cases; add abandoned-branch, session/cwd replacement, poisoned/aborted assistant, no-session, read failure, mkdir/write failure, and real command-path tests.

### 4.2 notify-pushover

Preserve tool `notify_human` and command `/notify-human-test`. Preserve one-way delivery, no automatic alerts, whole environment configuration taking precedence over a file, no mixing partial credentials, optional device, priorities -2 through 1, field limits, form encoding, and credential redaction.

Use PiG-specific text and `PIG_PUSHOVER_USER_KEY`, `PIG_PUSHOVER_APP_TOKEN`, and optional `PIG_PUSHOVER_DEVICE`. Use `<selected-agent-dir>/notify-pushover.json`, respecting PiG's agent-directory override. Retain the original JSON credential field names. Do not read the old Pi/kmet files, accept the old unprefixed variables as aliases, or copy credentials.

Use cancellable HTTP and a fixed Pushover endpoint. Retain kmet's 15-second timeout and redirect refusal, as explicitly approved by the user. Never retry a potentially accepted notification automatically. Keep secrets out of logs, test recordings, source and returned errors. Factory construction and registration validation must not send anything.

Acceptance: encode the original credential precedence, malformed config, late configuration, UTF-8/length boundaries, cancellation before/during send, status/body failures, redaction and reload cleanup behavior; use kmet cases as additional probes. Real delivery is a separate explicitly approved manual check. Keep `notify_human` and `/notify-human-test` unchanged despite the corrected extension directory name.

### 4.3 codex-usage

Preserve `/codex-usage`, `/codex-reset [reset-id]`, remaining-quota percentages, weekly-window tolerance, twenty-cell bars, local reset times, expiry ordering, native WHAM endpoints, and adapter `/codex/*` endpoints. Resolve native OAuth and adapter auth/headers through PiG, not auth-file parsing.

Own polling at five-minute intervals and refreshes on session start, model selection and agent settlement. Cancel obsolete HTTP work and reject late responses after model/session replacement or shutdown. Automatic failures clear only this status key and do not spam the transcript.

Use native PiG notifications for explicit command output. Consecutive info notifications may replace each other; append-only, nonpersistent output is not required. Do not emulate it with persisted custom messages. Retain immediate `/codex-reset [reset-id]` redemption without adding a confirmation step. This authorizes the feature, not real account mutations during development.

Acceptance: native OAuth, adapter/API-key and configured-header shapes; missing model/auth; finite-number validation; HTTP errors; reset request body and accepted results; timezone/date boundaries; stale replies; timer teardown; repeated command output; no credential leakage. Do not redeem real credits in tests.

### 4.4 pins

Preserve `/pin`, `pick`, `show`/`list`, `rm`, `clear`, and `help`; case-insensitive parsing; free labels; ten recent nonempty assistant candidates; stable candidate identity; auto-label truncation; and branch-local snapshots.

Use custom `pin-state` entries and restore only snapshots on the active branch. A raw PiG entry list contains entry wrappers, unlike kmet's facade. Do not assume `GetEntries()` is branch-filtered. Report corrupt snapshots without erasing them. Use the original Pi extension for command/model semantics and kmet for presentation. Retain regression checks for help-command dispatch and immutable open-view snapshots; review any additional kmet-only semantic change before adopting it.

The target browser is borderless, full width, top anchored, and 80% of terminal height. Preserve scrolling, pin switching, top/bottom keys, close keys, Markdown/code/table rendering, and no permanent widget. Capture session data off the input/render loop. Close owned UI on session changes and shutdown.

Acceptance: translate all model/core/UI tests; add actual host focus, narrow/tiny terminal, resize, theme, selection before first paint, duplicate labels, tree navigation, resume/reload and large-history tests. `HasUI` alone is insufficient: RPC can report UI availability without a terminal browser, so check mode for terminal-only operations.

### 4.5 imgview

Preserve `show_image` and `/imgcat`, `/imgshow`, `/imgboth`; path/URL/data-URI inputs; magic-byte MIME detection; literal `+` in data URIs; current cwd and home expansion; optional captions; seven accepted MIME types; and explicit browser opt-in.

Return ordered PiG text/image content for terminal tool results. Preserve slash-message summary/image-metadata separation and explicit no-new-turn delivery. Qualify the custom image renderer separately from tool-result rendering. Let PiG apply its own supported image normalization and terminal capabilities rather than reproducing kmet's missing conversion/iTerm2 support.

Preserve honest partial success: browser-only launch failure is an error; `both` can retain a useful inline image with an error note. Use private self-contained HTML, escaped metadata and argv-based launching. Never launch a browser while registering or validating an extension.

Retain the existing 8 MiB warning rather than inventing a new extension hard limit, and retain private browser-viewer files after unload so an opened viewer remains usable. Clean temporary downloads as before. Use PiG's native conversion and terminal capabilities. The host's 128 MiB wire-frame ceiling still applies and includes base64 and JSON overhead; detect an unsendable result before transport and report it honestly. Do not describe the source's fully buffered loading as bounded-memory support.

Acceptance: translate codecs/loading/browser/core cases; test redirects, cancellation, misleading MIME, malformed payloads, large input, transport-size boundaries, viewer privacy/cleanup and direct-terminal image/fallback paths. PTY/tmux text tests do not prove actual Kitty/iTerm2 graphics or a desktop opener through Lima.

### 4.6 schedule

Use original pi-schedule v0.4.0 at the pinned commit as the scope reference. Include interval, daily local-time and one-shot schedules; all four kinds (`prompt`, `shell`, `notify`, `message`); and all eight actions (`create`, `list`, `cancel`, `enable`, `disable`, `run_now`, `history`, `trust`). Preserve `once`, `maxRuns`, shell timeouts, `wakeOn`, global/project scope, missed-window rules, creation/fire caps, history and notices. Do not add cron syntax, a daemon, a dedicated slash command, or a child-agent runtime.

**Prompt delivery is current-session delivery.** Original `src/runner.ts:sendNow` calls `pi.sendUserMessage`, just like the kmet implementation. Neither creates a new Session nor filters existing history. Original `skills/schedule/SKILL.md` nevertheless claims “no prior conversation.” Follow the implementation and fix that claim in the adapted skill: require self-contained tasks, but do not promise history isolation. Shell follow-ups also enter the current Session. `delivered` records submission/action execution, not successful completion of an agent task.

**Preserve the original policy, including its explicit relaxed options.** Project jobs require scheduler trust for automatic firing. Explicit `run_now` bypasses that auto-fire gate, subject to the calling turn's privilege. Interactive project-job creation can grant trust as in the original; scheduled turns must not grant it implicitly. Global jobs are unaffected by the project gate. Preserve strict read-only as the default, the original known-read tool set, `suggest` permitting edit/write while blocking known execution/peer tools, shell jobs forcing `mutate`, and the explicit legacy read-only mode. Keep mutating `schedule` actions blocked in read-only/suggest turns and preserve early termination for fully blocked batches. Do not add confirmation gates or silently remove the relaxed mode. Bind policy meaning to PiG tool identities so another known shell tool cannot accidentally bypass a no-shell tier; review any incompatible mapping before landing. Q13's exclusion of BTW child-extension allowlists does not remove the scheduler's read-only tool allowlist.

Use `PIG_SCHEDULE_PRIVILEGE_MODE=legacy` for the relaxed opt-in and `PIG_SCHEDULE_SHELL` for the shell override. Use fresh stores under `<config-root>/state/schedule/` (`schedules.json`, `runs.jsonl`, `trusted.json`, `locks/`) and `<cwd>/.pig/schedule.json` for project jobs. Do not read `~/.pi-schedule`, `.pi/schedule.json`, or kmet EDN. Preserve the original cwd-only project scope and session cwd for global shell jobs. Define one current JSON/JSONL shape, with no automatic import or migration.

Port the pure schedule, action, policy, prompt-sanitization and redaction logic before the runner. Preserve wall-clock scheduling versus monotonic waits, the 30-second idle ticker, compaction-aware submission, atomic stores, locked read-modify-write, idempotency records, run ledger and corrupt-store diagnostics. Include the skill only after it describes proven behavior.

Source-review findings and feasibility obligations:

- Original `src/cli-prompt.ts` uses the host's `process.argv` and Pi parser to suppress automatic due checks only on process startup with an initial prompt/file argument. A Go factory's `os.Args` describes its generated runner, not the parent PiG invocation. The reviewed shared wire state exposes no equivalent initial-prompt field. Prove a supported host path before promising this behavior; do not inspect `/proc`, reimplement the CLI parser, or guess from empty history. Report a required host change separately.
- Original `src/runner.ts:processOne` enters privilege only after submission; `src/privilege.ts` pops on every agent settlement. kmet adds exact-prompt reservations. This difference is investigation evidence, not permission to copy either algorithm blindly. Preserve the policy's promised ownership of each scheduled turn, and red-prove unrelated settlement, multiple follow-ups, submission failure, abort and replacement before choosing the Go state machine.
- Original submission waits through compaction, including cancelled compaction without an end event. Port the observed ordering, bounded wait and errors rather than merely copying timer durations. Own cancellation and prevent delivery after unload/session replacement.
- The source's idempotency/lock approach is not an exactly-once transaction. Verify concurrent disable/cancel against fresh rows so stale completion cannot overwrite a disable or resurrect a cancelled job. Do not inherit a production-readiness claim from pure tests.
- Preserve original notices and session records for notify/message/shell actions with explicit no-new-turn delivery. Redacted persisted shell summaries do not make the live shell follow-up secret-free.

Acceptance requires original Pi test cases plus kmet's useful regressions: deterministic clocks, DST boundaries, startup initial-message/file suppression, `/new` and `/resume`, all kinds/actions, strict and legacy policy, trust grant/bypass, compaction success/cancellation/timeout, corrupt stores, concurrent processes, stale-lock ownership, cancellation while waiting/submitting/running, concurrent disable/cancel preservation, exact queued privilege ownership, no post-reload deliveries, process-tree termination and captured-output behavior. Reproduce `queued-tiers-activate-only-for-their-own-turn` and `stale-attempts-preserve-disable-and-never-resurrect-cancelled-jobs` at the PiG caller boundary.

The scheduler runs only while its selected PiG instance runs. Shell jobs and agent turns use the process's permissions. Tool policy is not an OS sandbox or protection against malicious installed extensions. Preserve these limitations in user documentation.

### 4.7 btw

BTW remains conditional, behind the six source-bearing ports. Target the reviewed original pi-btw v0.7.1, not v0.5.0. Prefer Go and assess whether PiG's existing public coding SDK makes a child-session design substantially more straightforward than kmet's missing substrate. Node's working `createAgentSession` export is useful evidence, not approval to introduce a Node implementation. A new shared child-session API or substantial host UI change is separate proposed work, not an implicit extension task.

Cover contextual/tangent/read-only modes, `/side`, built-in tool sets by mode, independent auth/model/thinking overrides, hidden branch-local threads, `--save` exclusion from parent context, follow-up injection, tool-free summarization, serialized submission/abort settlement, focus shortcuts, width toggling and headless behavior.

Exclude `btw.json` child-extension allowlists and child-extension loading. This means no ambient/parent extension discovery in the child, not “load every extension without a filter.” Retain the original built-in tool restrictions for read-only mode. This matches the original no-config path in `createBtwSubSession`, where the resource loader contains no child extensions. Document the omitted v0.7.1 extension-loading feature as an approved product-scope difference.

Before implementation, prove a Go design for child Session ownership, parent provider/auth inheritance, tool execution, context seeding, cancellation and floating UI. Use the existing public coding SDK or an approved host boundary; do not implement a second provider loop, use a one-shot completion, or silently drop modes. Defer BTW if this needs disproportionate new substrate.

Acceptance includes captured provider requests proving parent/child context isolation, built-in child tools and cancellation, overlay focus while the parent continues, reopen/resume/reload, credential fallback, no child-extension execution and no orphan child work. Adapt the existing PiG BTW replay to the selected Go extension and retain the source version/digest in its evidence. Existing Node replay results alone do not qualify a Go port.

### 4.8 extension-toggle

Do not implement this extension. Use `pig config` for user-scoped resource filters and `pig config --local` for project-scoped filters. Keep PiG's existing settings semantics. Do not introduce a package `enabled` field, copy its resolver, or create another settings-management plane.

## 5. Go-native repository and delivery model

Use an independent Go module and importable `Extension() *sdk.Extension` factory per extension. This is PiG's conventional Go source form: PiG generates the runner, and each exact directory can be validated and loaded independently. A `package main` executable, dynamic Go plugin, authored extension manifest, Node package manager, or root Package inventory is unnecessary.

Planned layout; only `plan.md` and Git metadata exist today:

```text
pig-extensions/
  plan.md
  README.md
  AGENTS.md
  LICENSE                       # MIT for new work; Jarkko Saltiola
  docs/
    provenance.md               # retains third-party notices, including ISC
    behavior-decisions.md
    validation.md
  extensions/
    savelast/
      go.mod
      extension.go
      extension_test.go
      README.md
    notify-pushover/             # same independent module/factory structure
    codex-usage/
    pins/
    imgview/
    schedule/
      go.mod
      extension.go
      skills/schedule/SKILL.md
    btw/                        # only after the conditional Go design qualifies
  test/
    integration/
    fixtures/
```

Use `github.com/jasalt/pig-extensions/extensions/<name>` as each module path. Pin dependencies per module. Add shared helpers only after two real callers need the same behavior. Keep tests and user instructions beside each extension. A local temporary Go workspace may bind these modules to this PiG checkout for tests; do not commit machine-local `replace` directives or SDK paths.

Document exact-root development and selective installation. Keep the schedule skill beside its owner and select it explicitly; do not assume loading a Go factory also discovers a skill. If an independently installable Package envelope is needed to carry both, qualify a per-extension manifest with exact members. Do not add an umbrella bundle or wildcard that activates other or unfinished extensions. Do not create a Piglet/Binary target without a later request.

Use namespaced extension-owned state under PiG's selected config root where appropriate. Keep session-specific pins and BTW threads in session entries. Resolve the selected agent directory for agent-scoped configuration rather than assuming `ConfigHome()/agent` always applies. Sections 4.2 and 4.6 specify notifier and scheduler paths.

No Pig-owned format version, compatibility reader, automatic migration, or kmet importer is included. Never import scheduler trust or secrets. Cross-SDK implementation is required if a shared PiG host capability changes, not for each ordinary Go extension product.

## 6. Implementation sequence

### Milestone 0 — Record decisions and source contracts

- Record the user's decisions in §8. This is complete, including the `jasalt` namespace, Jarkko Saltiola copyright holder, and notifier transport hardening.
- Initialize local Git on `main`. This is complete; no remote or commit is created.
- Freeze the original Pi source for each selected candidate and identify the approved differences. Do not treat uncommitted kmet worktree material as the primary reference.
- Retain original licenses and the user's local-notifier authorization. Do not relabel ISC material as MIT.
- Target the reviewed PiG checkout and Go SDK. Build a test binary from it before runtime acceptance rather than assuming the installed binary is identical.
- Check only the chosen toolchains. Go is available. No Node installation is needed for the selected production design; ask before installing tools for any oracle probes.

Exit: confirmed project identity and source contracts for the first port. Resolve later per-extension source conflicts before implementing the affected behavior.

### Milestone 1 — Minimal project and high-risk feasibility checks

- Add only the agreed project metadata, factory roots and hermetic test harness.
- Validate a minimal selected factory without installing it.
- Prove raw branch reads, non-triggering message delivery and reload cleanup through the real host.
- Probe pin geometry/Markdown and custom-image rendering before committing to their Go UI design.
- Probe scheduler startup-prompt metadata early, before implementing automatic delivery. Assess Go BTW child-session feasibility without expanding its conditional scope.
- Resolve blockers through existing APIs, an approved host change, an approved behavior difference, or explicit deferral. Do not bury a blocker in a renderer workaround.

Exit: each selected implementation route has a small production-path proof or an explicit blocker.

### Milestone 2 — Small independent ports

Port `savelast`, then `notify-pushover`. The initial savelast factory and pure tests are now present; complete its command/error matrix and retain real active-branch RPC evidence before moving on. Encode original Pi source behavior and approved differences, reuse kmet regression cases where applicable, and add real PiG command/tool tests. Retain the approved notifier transport hardening. Keep notification delivery offline. Establish patterns for configuration errors, current-session reads, cancellation and registration identity.

Exit: both work alone and together across reload without unintended external effects.

### Milestone 3 — Quota and interactive utilities

Port `codex-usage`, then `pins` and `imgview` after their feasibility gates. Own timer/HTTP/UI lifetimes. Retain exact output/state evidence for the agreed behavior differences.

Exit: functional, persistence, cancellation and terminal tests pass; unresolved graphics/desktop prerequisites are reported separately, not treated as passing.

### Milestone 4 — Scheduler reliability

Port pure modules, then stores/locks, then delivery and lifecycle. Qualify privilege association and startup-prompt suppression before allowing automatic prompt/shell execution. Exercise multi-process stores and actual shell cancellation. Do not mark the scheduler accepted until its full selected contract passes.

Exit: no known trust, privilege, duplicate-delivery, cancelled-job resurrection or resource-leak defect remains hidden behind unit-only coverage.

### Milestone 5 — Conditional BTW

Implement Go BTW only if its child-session/provider/UI feasibility design qualifies under §4.7. Track any proposed PiG changes as separate host work. A new shared host capability follows PiG's protocol → host → all applicable SDKs → conformance → parity documentation workflow. Do not implement `extension-toggle`.

Exit: BTW passes its actual Session/UI paths or has an explicit deferred disposition with the concrete blocker. The six priority extensions do not depend on BTW.

### Milestone 6 — Combined qualification and documentation

Validate the complete selected set. Test coexisting status keys, tool/command/shortcut conflicts, multiple UI users, fresh sessions, tree changes, reload, disconnect and shutdown. Test isolated and packed placements. Test fused Go only if it is a delivery target.

Write installation, minimum tested version, required tools, config/state paths, external effects, platform limitations and per-extension behavior differences. Publish nothing and alter no global installation without separate approval.

## 7. Verification and acceptance

### Test layers

1. **Behavioral baseline:** start from the pinned original Pi extension and its tests. Add the approved differences from §8 and kmet-style pin presentation. Use kmet tests as additional probes, not automatic authority when expected behavior differs. Record every intentional change.
2. **Pure unit tests:** parsing, text extraction, image MIME/codecs, quota math, pin snapshots, schedules, policies and store transitions.
3. **Boundary tests:** fake HTTP, clocks and opener/executor seams; cancelled operations; malformed responses; permission and persistence failures. Use real local HTTP servers where transport cancellation matters.
4. **Actual PiG path:** load the extension, invoke its command/tool, inspect session/provider effects, and drive reload/replacement/shutdown. Registration alone is not functional evidence.
5. **Terminal tests:** exact stable frame/cell comparisons for pins and conditional floating UI; width/theme/focus/input ordering; no transcript clearing or replay during ordinary popup changes.
6. **Stress and lifetime:** long sessions, large images/pins, polling overlap, queued scheduled jobs, repeated reloads, two PiG processes sharing stores, connection failure, process and file-descriptor cleanup. Record representative latency/allocation profiles for critical paths.
7. **Optional live checks:** Pushover delivery, Codex usage/reset, actual terminal graphics and desktop browser. These need explicit approval and their own evidence. Offline tests never claim them.

Use temporary homes, agent directories, sessions, project roots and stores. Never use checked-in source fixtures as writable homes. Use hermetic providers and dummy credentials. Do not let kmet and PiG schedulers share writable state. Disable ambient discovery in integration fixtures and select only the intended extension set.

Examples to finalize after scaffolding:

```sh
# Run inside each selected Go module, bound to the reviewed SDK checkout.
go test ./...
go test -race ./...
go vet ./...

# Set PIG_BIN to the isolated test binary built from the reviewed PiG checkout.
# From pig-extensions; resolves/builds/registers, but does not install.
"$PIG_BIN" install --validate-only --json ./extensions/savelast
"$PIG_BIN" install --validate-only --json ./extensions/savelast ./extensions/pins

# Validate a per-extension Package only if one is needed and authored.
# There is no root umbrella Package to validate.
```

Test commands must use an isolated environment before executing session handlers. `--validate-only` can execute factory construction, so constructors must remain free of network sends, browser launches, scheduling and credential mutations.

If implementation changes PiG core, run its required generation, lint, unit, conformance and relevant parity gates in `../PiG`. Changes confined to this new repository do not justify regenerating or editing unrelated PiG ledgers.

An extension is accepted only when its agreed source cases, real host behavior, cancellation, cleanup and persistence pass; its dependencies/licenses are recorded; its installation path validates; and every deferred platform or behavior is explicit. Do not weaken a test or silently reduce features to make a candidate appear complete.

## 8. Recorded user decisions

These decisions supersede the initial draft recommendations. The implementation implications below state how the answers are applied. Remaining technical feasibility gates are listed separately.

| ID | User decision | Implementation consequence |
|---|---|---|
| Q1 | Prioritize the six with source; consider the rest if simpler in PiG. | Five smaller utilities, then scheduler reliability. BTW gets a conditional Go feasibility review. Q14 excludes toggle. |
| Q2 | Prefer Go. | Independent native Go factories; no Node implementation or runtime dependency is assumed. Report a blocker before proposing another language. |
| Q3 | Original Pi extensions are the main reference; kmet pins has the better style. | Pin original source/tests. Use kmet pin presentation and approved differences; review other kmet-only semantics rather than inheriting them blindly. |
| Q4 | Target this checkout. | Use PiG commit `0e6ed0048282a15531ea1652a5833de4da4dd1a7` and its SDK for acceptance. Core-change permission was not given; propose any required host work separately. |
| Q5 | Independent extensions; choose the Go-native form. | One Go module and `Extension() *sdk.Extension` factory per exact root. No umbrella Package, Node tooling, or Piglet requirement. |
| Q6 | Linux-focused. | Linux implementation and actual terminal/browser qualification. Preserve honest headless behavior; do not claim other platforms tested. |
| Q7 | Last response on the active branch. | `GetBranch(nil)`, latest assistant text only, no fallback to an older text-bearing response. Other savelast semantics remain original, including overwrite. |
| Q8 | Native PiG notifications; retain immediate reset redemption. | Accept native status replacement. Keep `/codex-reset` without a new confirmation; tests never redeem real credits. |
| Q9 | Correct spelling; PiG-specific config and environment variables. | `notify-pushover`, `PIG_PUSHOVER_*`, `<selected-agent-dir>/notify-pushover.json`; no legacy aliases or secret migration. Tool/command names stay unchanged. |
| Q10 | Retain/use native capabilities. | Use native PiG conversion/terminal rendering. Retain warning-only image threshold and private browser viewers; respect actual transport limits. |
| Q11 | Use the Pi extension scope for the Go scheduler. | Original pi-schedule v0.4.0 kinds/actions/policy. Its code proves current-session delivery, not history-free child sessions. Correct the adapted skill accordingly. |
| Q12 | Preserve scheduler policy. | Keep trust behavior, explicit `run_now` bypass, strict default, drafting-capable `suggest`, and opt-in legacy mode. Do not impose new confirmation gates. |
| Q13 | Reviewed BTW version; no allowlist. | Conditional v0.7.1 port without configurable child-extension loading. No ambient child extensions. Keep built-in mode/tool restrictions. |
| Q14 | `pig config` is sufficient. | Exclude `extension-toggle`. No new package-enable schema or resource-management API. |
| Q15 | Fresh PiG state, MIT, initialize Git, local source authorized. | No importer or compatibility reader. MIT for new work with original third-party notices retained. Git initialized on `main`; no remote or commit. Repository/module prefix is `github.com/jasalt/pig-extensions`; copyright holder is Jarkko Saltiola. |

### 8.1 Confirmed project identity and notifier policy

The user selects the `jasalt` namespace and Jarkko Saltiola as copyright holder. Use repository identity `github.com/jasalt/pig-extensions` and module paths `github.com/jasalt/pig-extensions/extensions/<name>`. License new work under MIT and preserve upstream license notices. This identity does not authorize creating or publishing a remote repository.

The user also approves retaining kmet's 15-second notifier timeout and redirect refusal. Record these as deliberate differences from the original local Pi notifier, whose fetch uses caller cancellation and default redirect handling.

### 8.2 Remaining feasibility gates
- **Scheduler initial-prompt detection:** prove a supported parent-host metadata path; otherwise present the required host change for approval or mark the scheduler blocked. Do not silently drop startup suppression.
- **Pins/imgview rendering and BTW:** qualify existing Go APIs before committing to an implementation route. A blocked route does not authorize a host edit, Node fallback, or reduced behavior by itself.
- **Linux graphics/browser:** identify the actual terminal and desktop bridge during qualification. Text-only tests cannot establish graphics or desktop success.

## 9. Immediate next action

Complete the savelast command/error matrix and add the source/provenance notice, then port `notify-pushover`. Keep the source contract and failing regression cases ahead of implementation. Keep BTW conditional and do not build `extension-toggle`. Do not install extensions globally, publish, commit, or create a remote as part of Git initialization.

### Main PiG references

Paths are relative to this workspace:

- `../PiG/docs/extension-authoring.md`
- `../PiG/docs/extension-api-parity.md`
- `../PiG/docs/extension-runtime-cells.md`
- `../PiG/docs/site/docs/extensions.md`
- `../PiG/docs/site/docs/development.md`
- `../PiG/extensions/sdk/{context.go,session_manager.go,model_registry.go,extension.go}`
- `../PiG/coding/extension/host/subprocess/protocol.go`
- `../PiG/coding/extension/host/subprocess/runtime-node/shims/pi-coding-agent.mjs`
- `../PiG/coding/extension/host/subprocess/runtime_node_child_session_test.go`
- `../PiG/coding/extension/host/subprocess/runtime_node_child_lifetime_test.go`
- `../PiG/internal/codingagent/{settings.go,ext_ui_context.go,interactive_chat.go}`
- `../PiG/docs/findings/fix-overlay-shim.md`
- `../PiG/test/parity/unit-evidence/replay-node-btw.py`
- `../PiG/examples/extensions/go-factory/`

The local materialized reference at `/home/user.guest/.pig/docs/` was also consulted for extensions, API, packages, SDK, events, TUI, keybindings, terminal setup and divergences. It is read-only and does not override current production source.
