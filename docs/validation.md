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
source hash `81bd5d48214d96c53b001f65942308a41e3c1e6e421bdd12b9fbb0da427da1fe`).
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

Fresh active LSP checks are clean for the Python integration files (explicit
`pyrightconfig.json` refreshes discovery of newly-created helpers). Go unit,
race and vet checks provide additional compiler/runtime evidence. Reload in
an interactive terminal and packed placement still belong to combined qualification;
these are not claimed from process restart or RPC-only tests.

## Remaining scope

The remaining four priority ports, rendering feasibility, scheduler metadata/policy
qualification, combined placement/lifetime tests and conditional BTW assessment
remain unaccepted. Text-only terminal tests will not be presented as real graphics
or desktop-opener proof. See `plan.md` for the full remaining acceptance matrix.
