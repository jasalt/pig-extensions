# Validation evidence

Target: Linux amd64; PiG checkout `0e6ed0048282a15531ea1652a5833de4da4dd1a7`,
binary `../PiG/bin/pig`, version `0.3.1+0.87.1`. No PiG core modifications.
Tests use temporary homes, selected agent directories, cwd roots and sessions.
They never send real notifications, call real providers, or redeem credits.

## savelast

Passed:

```sh
(cd extensions/savelast && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.savelast
python3 -m test.integration.savelast_mutation
```

Pure tests encode original block extraction/byte preservation, normalization,
parent creation, overwrite, latest-textless and missing-content behavior.
The missing-content case matches original JavaScript undefined, whereas null
is a present textless content. Error/aborted assistant text is not filtered.
ECMAScript whitespace (including BOM but excluding NEL) matches the original;
real RPC proves a BOM-only response warns while a NEL response is saved exactly.
Pinned Go 1.27.1 unit/race/vet checks also pass.

Real RPC verifies exact active-branch bytes with a newer abandoned assistant
in the full log; default/blank, relative/absolute, spaces/Unicode, overwrite,
mkdir/write failure notifications, no assistant, textless assistant, ephemeral
sessions, `/new` equivalent and switch-session/cwd replacement. The driver
waits for command registration and each command response before continuing.
Its notification comparisons use the actual RPC `notifyType` field.

The whole-log mutant compiles and passes pure tests, but fails exact RPC bytes
with `ABANDONED` instead of the active response. No original files are mutated.

A socket-pair SDK boundary test injects `sessionRead` failure and verifies one
error notification, a successful command response (no duplicate generic error)
and clean shutdown. This is failure-injection evidence, **not** a failure observed
in production PiG. `SessionManager.GetBranch(nil)` uses the host's `sessionRead`
method, not the convenience mirror's `watchSessionLog` subscription.

Registration validation passed (`valid: true`, `registered: true`, Go factory,
source hash `041ba01f0ddbe782b9d176bca8f8404a76f9bb406b4d115abd887cd99b238fd4`).
It is a separate gate and does not prove functionality.
No global installation or remote publication was performed.

## notify-pushover

Passed with Go 1.27.1 (the pinned, already-installed toolchain) as well as the
current Go 1.26.8 default:

```sh
(cd extensions/notify-pushover && GOTOOLCHAIN=go1.27.1 go test ./... && GOTOOLCHAIN=go1.27.1 go test -race ./... && GOTOOLCHAIN=go1.27.1 go vet ./...)
python3 -m test.integration.notify_pushover
python3 -m test.integration.notify_tool
python3 -m test.integration.notify_cancel
```

Core/boundary tests cover whole-environment precedence, no partial mixing,
missing/malformed/late config, original JSON coercion, selected directory rules,
ignored legacy aliases, UTF-16/form limits including WTF-8 surrogate input,
ECMAScript whitespace, redaction of raw/component/form-encoded credentials,
accepted/error responses, body-read cleanup, timeout, no retry and cancellation
before/during HTTP with a real local server.

The actual command path uses the unchanged production fixed HTTPS endpoint,
a local CONNECT proxy and temporary trusted fixture certificate. It proves
missing/late configuration, exact POST form, redirect refusal and redaction,
new-session credential refresh and coexistence with savelast. The actual tool
path uses a local OpenAI-compatible streaming provider; it proves model tool
selection, dispatch, exact form, ordered result content/details, and delivery of
the tool result back to that provider without credentials in the result/context.
No test contacts real Pushover or an external provider.

A held HTTPS response proves actual host abort/disconnect closes the in-flight
connection within five seconds, never retries and leaves a cleanly exited host.
An initial driver failure exposed `agent_end` arriving before the abort response;
the corrected driver searches the marked event window rather than waiting for
a duplicate settlement event. The HTTP disconnect assertion remains mandatory.

Registration validation passed (`valid: true`, `registered: true`, Go factory,
source hash `7b11f0fdc928655727db648e65924c305a5ae1e598cd95a26320adae07a1bb87`).
This separately proves tool/command identity and an effect-free constructor,
not runtime acceptance by itself.

Fresh active LSP checks are clean across 15 changed Go/Python files (explicit
`pyrightconfig.json` refreshes discovery of newly-created Python helpers). Go unit,
race and vet checks provide additional compiler/runtime evidence. Reload in
an interactive terminal and packed placement still belong to combined qualification;
these are not claimed from process restart or RPC-only tests.

## codex-usage — offline command/lifecycle caller evidence

Passed:

```sh
(cd extensions/codex-usage && GOTOOLCHAIN=go1.27.1 go test ./... && GOTOOLCHAIN=go1.27.1 go test -race ./... && GOTOOLCHAIN=go1.27.1 go vet ./...)
python3 -m test.integration.codex_usage
python3 -m test.integration.codex_native
python3 -m test.integration.codex_lifetime
```

Original-source cases are encoded for finite window validation, weekly tolerance,
clamping, primary/weekly identity, twenty-cell bars and exact percentage rounding,
local reset times/DST/time clipping, expiry ordering, UTC credit timestamps,
accepted reset acknowledgements and exact reset-ID parsing. Kmet's current
regression cases were consulted (checkout `ab36bf230cbec7119177bba717e511dd9b4c0ccc`),
not treated as authority for its different notification or Pi identity behavior.

HTTP seams/real local servers prove adapter paths/auth/header merging, native JWT
claims/WHAM paths and unique v4 redemption IDs, accepted/error/malformed results,
credential redaction, cross-origin header stripping and cancellation without
retrying a possibly accepted reset. Those native HTTP seams are not themselves
native OAuth caller-path evidence; that separate proof is described below.
Lifecycle tests prove overlapping refresh suppression,
epoch replacement, pending-metadata races, shutdown joins, timer teardown,
normal-handler versus runtime lifetimes, silent automatic failures and immediate
reset followed by refresh. Malformed truthy reset results cannot become success.

Real PiG RPC uses a local adapter and hermetic model. Exact repeated cards,
expiry-sorted list output, reset POST JSON/headers, invalid-ID no-send, session start,
model selection and real-agent settlement refreshes pass. Notifications are not
persisted custom messages. A fresh PiG RPC home exposes an `unknown/unknown`
placeholder, so its exact diagnostic is missing authentication for `unknown`;
a genuinely absent model is separately covered at the owner boundary. The
settlement test waits for the specific usage GET with a deadline because
`agent_end`/command responses are not settlement barriers. No account mutation
or real provider/WHAM request was made.

Native real-host proof seeds an isolated PiG OAuth store with an unexpired dummy
JWT and lets PiG resolve it. Production fixed WHAM endpoints are intercepted by a
private local TLS proxy. Account/plan claims override server account data; headers
include the exact resolved bearer token, account ID and PiG originator. All four
accepted results plus empty-object acknowledgement pass, with unique v4 UUIDs in
native redemption bodies. Unexpected results neither retry nor refresh. No real
OAuth/login or credit mutation is performed.

Held-response real-host tests verify model selection, new-session replacement and
disconnect close obsolete HTTP sockets within five seconds, suppress old notices
and footer status, and leave replacement epochs usable. Buffered shutdown RPC
frames are retained for credential/stale-output checks rather than discarded.
The harness forces PiG directory mode, both agent overrides and offline catalogs;
ambient shared-mode settings cannot redirect test credential reads to live Pi
state. TLS seams are shared only now that both Pushover and Codex need them.

Factory registration validation passes independently (`valid: true`,
`registered: true`, source hash
`53b0eb00661ef252ce478d845ec390ada63999f533eafeef2cf3865fcb425d3b`),
with both commands and all four lifecycle handlers. Interactive reload and
combined placement remain required; no full combined/terminal acceptance is
claimed from these RPC proofs. Fresh active LSP checks are clean for the eight
changed/shared Python caller fixtures.

## imgview

Passed (Go 1.26.x default and pinned `GOTOOLCHAIN=go1.27.1`):

```sh
(cd extensions/imgview && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.imgview
python3 -m test.integration.imgview_pty
```

Unit tests encode all upstream `utils.test.ts` cases plus BMP/AVIF/SVG
whitespace sniffing, misleading declared types, Node-forgiving base64, literal
`+` and malformed percent payloads, cwd/`~`/non-regular/missing files, HTTP
redirect/404/declared-type/connection failure, cancellation during a held
download body and before a file read, viewer escaping/uniqueness/`0700`/`0600`
modes, and argv-only opener launch with a missing-executable error.

Real host RPC (reviewed binary): `/imgcat` persists exactly one display
`imgview-image` message with base64 details and starts no turn; `/imgshow`
writes a private viewer and invokes a fake `xdg-open` with its path; data URI
`/imgboth`; usage/unsupported/missing/directory/malformed errors; missing opener
gives only an error for `/imgshow` and keeps the inline message for `/imgboth`.
A hermetic image-capable model calls `show_image`: the tool update, ordered
text+image result, caption, model delivery of the image, browser-only text
result without base64, browser launch failure as an error, `both` partial
success, and non-image/missing-file rejections pass.

Interactive PTY (reviewed binary): the `/imgcat` renderer emits Kitty protocol
bytes with the image data under a Kitty environment, and PiG's native
`[image/png] 2x2` fallback under a plain terminal or `PI_IMAGE_PROTOCOL=none`.
This is terminal-stream evidence only, not proof of a displayed graphic. No
real desktop browser launch was attempted.

Registration validation passed (`valid: true`, `registered: true`, Go factory,
packable, source hash
`d2e907a4113853e9f6d25502575cc5d3ecc23c40b44f311ed7232953d2518e10`).

## pins

Passed (Go 1.26.x default and pinned `GOTOOLCHAIN=go1.27.1`):

```sh
(cd extensions/pins && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.pins_rpc
python3 -m test.integration.pins_pty
```

Unit tests translate kmet's model/UI cases and the original semantics: text
extraction, auto-label/preview with UTF-16 lengths and ECMAScript whitespace,
ten-candidate window with textless distance, last-snapshot restore, `nextId`
repair, ten corrupt-state shapes, command parsing including free labels and
the restricted `list` alias, overlay geometry, layout invariants for heights
1–79, full-width borderless rendering, scrolling/switching/`g`/`G`, Kitty
CSI-u keys, resize/tiny terminals, native Markdown table/code, close keys
before the first frame, snapshot immutability and disposal, and the
lexer-to-theme token mapping.

Real host RPC (reviewed binary) with a seeded branched session: a pin that
exists only on an abandoned branch is not visible; `/pin`, `pick` through an
RPC select response (exact title/options, cancel adds nothing), labels,
`list of plugins` label, `rm`/usage/unknown errors, `show` warning outside the
TUI, `help`, exact persisted snapshots, restart persistence, `clear` with ID
restart, new-session empty branch, and corrupt-state refusal for every
state-reading command with the corrupt entry left intact.

Interactive PTY (reviewed binary) with a custom theme whose `syntaxKeyword`
is `#123abc`: `/pin show` mounts the overlay; the Go keyword is emitted with
that exact host color; the heading is rendered, the table keeps its borders;
`G`, `PgDn`, `PgUp` change the frame; a width change re-renders; `q` and `Esc`
close and the editor accepts the next command; `/pin list 7` preselects.
This closes the host-theme highlighting counterexample recorded in
`docs/pins-rendering-evidence.md` for this extension. Exact screen-cell
composition, height-only resize and hyperlink-setting propagation are not
claimed.

Registration validation passed (`valid: true`, `registered: true`, Go factory,
packable, source hash
`418380f23262f591d0adb12921bcb382f1a8de9cc1b12d8db9c4242fc9025c2e`).

## schedule

Passed (Go 1.26.x default and pinned `GOTOOLCHAIN=go1.27.1`):

```sh
(cd extensions/schedule && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.schedule_rpc
```

Unit tests translate the original suite: parsing (interval/daily/once forms,
bounds), next-run computation including America/New_York spring-forward and
fall-back (earlier instant), relative formatting, missed-window grace, rate
limiter, kind/wakeOn/timeout/maxRuns validation, follow-up selection,
truncation, prompt contract with fence/ANSI/C1 break-out cases, redaction
(token shapes, schemes, quoted/unquoted assignments, idempotence). Store tests
cover provenance relabeling, foreign-row coercion/clamping, seven corrupt
shapes quarantined byte-for-byte, fresh/stale store locks, caps, concurrent
writers, default scope, trust fail-closed cases, job locks (stale takeover,
victim release) and ledger idempotency window/rotation. Runner tests with a
fake session cover firing, idempotent replay, `run_now`, trust gate and
notices, caps/follow-ups, busy ticks, skip/error advancement, lock contention,
wave serialization, concurrent disable/cancel during delivery, `once`/`maxRuns`
termination, notify/message/shell kinds with redaction, compaction wait
(success, timeout, busy retry, shutdown), store error cooldown, the privilege
matrix, and **queued-tiers-activate-only-for-their-own-turn** and
**stale-attempts-preserve-disable-and-never-resurrect-cancelled-jobs**. A real
process-group test proves a timed-out job's background child is killed.

Real host RPC (reviewed binary, scripted local model):
- no wave at process startup; `/new` fires a due read_only prompt; the
  model's `bash` call is blocked with the scheduler reason, the batch
  terminates (one provider request), the store advances and the ledger
  records `delivered`; the user's next own turn runs `bash` unrestricted;
- two jobs due together (read_only then a queued mutate follow-up in the same
  run): the first turn's `bash` is blocked, the second's runs;
- a shell job with a 1.5 s timeout and a background `sleep 30` is recorded as
  killed, its child process is gone, and persisted output is redacted; an
  untrusted project shell job is held back with a notice and never runs;
- the model creates a notify job through the tool; the skill is written
  under the state directory and offered as `skill:schedule`.

Registration validation passed (`valid: true`, `registered: true`, packable,
source hash `f27f7eaba826f57d201a36088306e8a304a0a277f34503eeab83f6fb6a6a7bbd`)
and created no store or skill files. Not covered: the 30 s idle tick on a
live host (unit-tested only), interactive `/resume`, compaction against a real
compaction run, multi-process delivery races beyond file locks, DST in a live
session.

## Remaining scope

Codex's combined/terminal gates and the other three priority ports, rendering
feasibility, scheduler metadata/policy
qualification, combined placement/lifetime tests and conditional BTW assessment
remain unaccepted. Text-only terminal tests will not be presented as real graphics
or desktop-opener proof. See `plan.md` for the full remaining acceptance matrix.
