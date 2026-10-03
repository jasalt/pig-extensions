# Unfinished-extension feasibility gates

Target source remains PiG commit `0e6ed0048282a15531ea1652a5833de4da4dd1a7`.
No sibling source, core settings or global installation has been modified.
An unresolved gate is not runtime acceptance or permission for a reduced port.

## Scheduler startup metadata — host approval needed

Original pi-schedule v0.4.0 `src/cli-prompt.ts` uses the parent Pi parser:
`args.messages.length > 0 || args.fileArgs.length > 0`. It suppresses automatic
startup due checks only for `session_start.reason === "startup"`; new/resume/fork
sessions must not keep inheriting launch arguments. The plan requires this
behavior and forbids `/proc`, a duplicate CLI parser, or history-based guessing.

Current authoritative host boundaries inspected:

- `coding/extension/events.go:SessionStartEvent` carries `type`, `reason`,
  `previousSessionFile`, but **no initial CLI message/file metadata**.
- `coding/extension/host/subprocess/protocol.go:StatePayload` carries model,
  session, idle/trust/pending-message state, system prompt options, registered
  flags, UI/terminal/theme/keybindings state, but no initial CLI prompt/file flag.
- `extensions/sdk/protocol.go:readyMsg` carries the generated extension's
  host/session snapshot, not the parent invocation's argv.
- A Go factory's `os.Args` belongs to its generated runner, not the user's PiG CLI.
  `HasPendingMessages` is conversation queue state, not supported CLI metadata.

This is source-contract evidence for an unresolved supported metadata route,
not a successful live startup suppression test. No compatible route has been
qualified. Automatic prompt/shell delivery must not be advertised or accepted
by silently omitting this suppression.

### Proposed separate host work (not implemented or implicitly approved)

Expose an additive optional `hasInitialPrompt` boolean on the startup
`session_start` event, populated by PiG's existing CLI parser from initial
message/file argument presence. Keep the existing `reason` field; scheduler
suppression is `reason == startup && hasInitialPrompt`. Old/unknown host metadata
must remain distinguishable from false, rather than masquerading as no prompt.

If approved, qualify protocol/host and all applicable SDK event propagation,
CLI message and @file cases, no-prompt startup, and new/resume/fork/reload behavior
with the required PiG tests/documentation. This changes the reviewed host and
therefore also needs an approved target/version update. Alternatively, explicitly
defer the scheduler runtime while its pure schedule/policy/store logic is ported.
No Node fallback, confirmation gate or automatic scope reduction is proposed.

## Pins / imgview — probe still required

The shared host wire includes resolved terminal capabilities, but Go SDK
`Extension.handleNotify(state_update)` currently retains model, session, HasUI
and theme, not that capability payload. The Node runtime does propagate it.
This does not by itself prove the Go rendering route impossible: public native
Markdown/Image packages and host image-result normalization still need scoped
production-path probes. Do not infer parent capabilities from a child process
TTY or implement a competing renderer/protocol to hide a gap.

No pin browser geometry, Markdown/theme/focus path, slash-image renderer,
Kitty/iTerm2 graphics or desktop opener has been accepted yet. Ordinary text
PTY output and `HasUI` in RPC are not sufficient evidence.

## BTW — conditional after priority ports

No child-session design or implementation has yet qualified. Preserve the Go
preference, original v0.7.1 modes/tool restrictions and no child-extension loading.
Use a real public child-agent/session substrate or explicitly defer; never replace
it with a one-shot completion or a second provider loop.
