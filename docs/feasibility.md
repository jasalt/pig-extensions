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

## Pins / imgview — resolved for the shipped extensions

The Go SDK still drops the host's resolved terminal capabilities from
`state_update`, and PiG's public `tui.HighlightCode`/hyperlink rendering read
process globals. The shipped extensions handle this as follows (see each
README and `docs/validation.md`):

- pins highlights code with a per-instance copy of PiG's lexer mapping fed by
  the host theme snapshot (PTY-proven with a custom keyword color).
  Hyperlinks follow the extension process's capability detection, and a
  height-only resize is not redrawn: both recorded SDK gaps.
- imgview's tool images are host-rendered. Slash-command images use the
  public `tui.Image`, which in subprocess placement detects capabilities from
  the inherited environment (PTY-proven Kitty bytes and native fallback) and
  shares the host's state when fused.

Actual graphics on a physical terminal and a real desktop browser launch
remain unverified.

## BTW — child-session substrate proven; port not started

`test/fixtures/btw-child-session-probe` (run
`python3 -m test.integration.btw_child_probe`) shows that a Go extension
command can run a child agent turn through PiG's public `coding` SDK
(`NewServices` → `NewRuntime` → `BuildModel(ctx.ModelQualified())` →
`Runtime.New` with an in-memory session). Against the reviewed binary and a
scripted local model:

- the child resolved the parent's model and credentials from the selected
  agent directory;
- its request carried only its own system prompt and question (no parent
  history);
- its tools were exactly `read`, `grep`, `find`, `ls`
  (`ActiveBuiltinTools`); no extensions were loaded in the child runtime;
- cancellation follows the command's context.

This is the substrate the plan requires, using PiG's own agent loop rather
than a second provider loop. It does **not** qualify a BTW port. Remaining
work for pi-btw v0.7.1 (~3,100 TypeScript lines):

- contextual mode (seeding the parent branch), tangent and read-only modes,
  `/side`, `--save`, follow-up injection, tool-free summarization;
- hidden branch-local threads;
- a floating overlay that streams output and switches focus while the
  parent keeps working. The SDK overlay is single, line-based, and does
  not redraw on height-only changes (see the pins evidence);
- credential concurrency: in subprocess placement the child reads and may
  refresh OAuth credentials from the same agent directory as the host.
  Concurrent refreshes need qualification before relying on rotating tokens.

Recommendation: implement BTW as its own milestone with an overlay
qualification step first.
