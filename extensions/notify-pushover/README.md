# notify-pushover

Independent Go PiG factory. Provides tool `notify_human` and command
`/notify-human-test [message]`. Delivery is one-way: it does not collect a reply.
There are no automatic alerts. Invoking either capability with valid credentials
**sends a real notification**; registration and validation do not send anything.

## Configuration

Either set the whole credential pair:

```sh
export PIG_PUSHOVER_USER_KEY='...'
export PIG_PUSHOVER_APP_TOKEN='...'
export PIG_PUSHOVER_DEVICE='optional-device'
```

Or create `<selected-agent-dir>/notify-pushover.json` (prefer mode `0600`):

```json
{"userKey":"...","appToken":"...","device":"optional-device"}
```

The original nested `{"pushover":{...}}` shape is also accepted. A complete
environment pair takes precedence over the entire file, including the device.
Partial environment credentials are never combined with file credentials.
Credentials are loaded on session start; missing credentials are retried lazily
on invocation, so late configuration works. A new session refreshes the cache.

Selected directory rules match this PiG checkout: `PIG_CODING_AGENT_DIR` overrides
`<config-root>/agent`; tilde prefixes are expanded. Config root is `PIG_HOME`,
then `XDG_CONFIG_HOME/pig`, then `~/.pig`. When the user explicitly enables PiG's
shared-directory mode (`PIG_USE_PI_DIRS=1`), its selected Pi agent-directory rules
apply, but the file is still named **notify-pushover.json**. No old credential
file, legacy environment alias, or migration is used.

## Behavior and transport

- PiG-specific titles and status key; native notifications and keyed footer status.
- Priorities -2, -1, 0, 1. Message/title/URL/URL-title limits follow original UTF-16
  slicing (1024/250/512/100 units), including cut-surrogate replacement in forms.
- Fixed `https://api.pushover.net/1/messages.json` endpoint; standard Go proxy/TLS
  environment applies. Form encoding, caller cancellation, **15-second timeout**,
  **no redirects**, and no automatic retry of potentially accepted notifications.
- Credential values and their encoded spellings are redacted from returned errors.
  Malformed credential JSON diagnostics deliberately do not quote file contents.
- Shutdown cancels and joins owned sends, clears only this status key and closes
  idle connections. An aborted request can already have reached Pushover: an error
  is not proof of nondelivery, and should not cause an automatic retry.

## Development qualification

Tested against PiG checkout `0e6ed0048282a15531ea1652a5833de4da4dd1a7`, Linux,
SDK v0.3.1, Go language floor 1.26. These are source-root factories, not a bundle.

```sh
pig --no-extensions -e /absolute/path/to/extensions/notify-pushover
# Resolve/build/register without installation or delivery:
pig install --validate-only --json ./extensions/notify-pushover
```

From this repository:

```sh
(cd extensions/notify-pushover && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.notify_pushover
python3 -m test.integration.notify_tool
python3 -m test.integration.notify_cancel
```

Integration requires the test PiG binary (`PIG_BIN`, default `../PiG/bin/pig`),
Python 3 and OpenSSL. It uses dummy credentials, a hermetic model, temporary
certificate trust and a local TLS proxy; **no real Pushover delivery** is tested.
Real delivery needs separate manual authorization. See `../../docs/validation.md`
for evidence scope and `../../docs/provenance.md` for source attribution.
