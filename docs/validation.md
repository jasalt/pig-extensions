# Validation evidence

Target: Linux amd64; PiG checkout `0e6ed0048282a15531ea1652a5833de4da4dd1a7`,
binary `../PiG/bin/pig`, version `0.3.1+0.87.1`. No PiG core modifications.
Tests use temporary homes, selected agent directories, cwd roots and sessions.
They never send real notifications, call real providers, or redeem credits.

## savelast

Passed:

```sh
(cd extensions/savelast && go test ./... && go test -race ./... && go vet ./...)
python3 test/integration/savelast.py
python3 test/integration/savelast_mutation.py
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

## Remaining scope

The other five priority ports, rendering feasibility, scheduler metadata/policy
qualification, combined placement/lifetime tests and conditional BTW assessment
remain unaccepted. Text-only terminal tests will not be presented as real graphics
or desktop-opener proof. See `plan.md` for the full remaining acceptance matrix.
