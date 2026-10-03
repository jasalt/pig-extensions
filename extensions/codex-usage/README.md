# codex-usage

Independent Go factory for PiG, adapted from the pinned original Pi extension.
Provides `/codex-usage` and `/codex-reset [reset-id]`.

**Implemented with offline caller evidence.** Pure/HTTP/lifecycle tests, real PiG
adapter commands, native OAuth/WHAM commands and real-host HTTP/model/session
replacement/disconnect tests pass. Interactive reload and packed placement remain
combined-qualification gates, not claims made from unit tests or process restart.

## Behavior

- Remaining quota, twenty-cell bars, weekly windows within 5% of seven days,
  account details, local reset times and expiry-ordered banked reset credits.
- `/codex-reset` lists credits; `/codex-reset exact-id` immediately redeems that
  exact ID, without a new confirmation gate. **This is an account mutation.**
  Development tests use local dummy accounts only, never real credits.
- Native `openai-codex` uses WHAM endpoints and claims from the host-resolved OAuth
  access token. Adapter providers use their resolved base URL's `/codex/usage`,
  `/codex/resets` and `/codex/reset`. The extension does not parse auth files.
- Host-resolved model authentication and configured request headers are used.
  Native requests identify their originator as `pig` (PiG D26), not `pi`.
- Explicit output uses native PiG notifications, not persisted custom messages.
  Consecutive info output can replace prior native status, as approved.
- Poll every five minutes while PiG runs; refresh on session start, model selection
  and agent settlement. Automatic failures clear only the `codex-usage` status key
  and never notify/spam the conversation.
- Own transport/ticker/cancellation; superseded refreshes and old model/session
  operations cannot publish after their generation changes. Shutdown cancels and
  joins work. Completed handler contexts are not used as HTTP request lifetimes.
- HTTP timeout is 15 seconds. No automatic retry of reset redemption. Redirects
  remain supported, but configured authentication headers are stripped if origin
  changes. Raw/encoded credentials are redacted from errors and displayed data.

Reset errors or cancellation do **not** prove nondelivery. The account may have
accepted a redemption before the connection was closed; do not automatically retry.

## Selection and development

Target: Linux, PiG checkout `0e6ed0048282a15531ea1652a5833de4da4dd1a7`, SDK v0.3.1,
Go floor 1.26, qualified toolchain 1.27.1. Reset-time formatting has been checked
for en_US/en_GB and local timezone/DST boundaries; no blanket locale/platform
parity is claimed.

```sh
pig --no-extensions -e /absolute/path/to/extensions/codex-usage
pig install --validate-only --json ./extensions/codex-usage
```

Validation executes only an effect-free constructor. It does not start polling,
query accounts or redeem credits. Runtime session events do initiate usage queries.
A fresh unconfigured PiG RPC home exposes an `unknown/unknown` placeholder model;
its explicit error is `No authentication available for unknown`, not a nil-model
error. An actually absent model yields `No model selected`.

```sh
(cd extensions/codex-usage && GOTOOLCHAIN=go1.27.1 go test ./...)
(cd extensions/codex-usage && GOTOOLCHAIN=go1.27.1 go test -race ./...)
(cd extensions/codex-usage && GOTOOLCHAIN=go1.27.1 go vet ./...)
python3 -m test.integration.codex_usage
python3 -m test.integration.codex_native
python3 -m test.integration.codex_lifetime
```

Integration requires Python 3, OpenSSL and the test PiG binary (`PIG_BIN`, default
`../PiG/bin/pig`), a temporary selected agent directory, dummy credentials, a local
HTTP adapter/provider and private local TLS interception for the fixed WHAM
endpoints. The host reads only seeded dummy OAuth credentials; the extension reads
none of those files. It never contacts a live account. Shared-directory mode is
disabled and both PiG/Pi agent directory overrides are pinned to the fixture.
Settlement follows
`agent_end` asynchronously; the test waits for the specific usage request instead
of assuming a command response is a settlement barrier.

Original source: `jasalt/chatgpt-openai-api-adapter`, commit
`94f45568b4bd7842b1aef362cc3ba883b1312951`, `contrib/pi-codex-usage.ts`.
Source SHA-256: `9e6bf72c5e050b51d7825a667a351062afe6b81bb737507a9f4175e181cea3b7`.
Original MIT license is retained verbatim in `LICENSE`; new work is also MIT.
