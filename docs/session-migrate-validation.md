# session-migrate validation (2026-10-06)

Implemented scope: **Claude Code → PiG**, not upstream's full multi-harness matrix.
See [extension usage and limitations](../extensions/session-migrate/README.md)
and [source/license inventory](provenance.md#session-migrate).

## Independent real-host evidence

Clean PiG source checkout: `0e6ed0048282a15531ea1652a5833de4da4dd1a7`,
version `0.3.1+0.87.1`, Linux amd64. Go extension checks use Go 1.27.1.
A stock binary was built from that checkout; the root seven-member Piglet was
also built with `extensionRealization: fused` from the same checkout.

Passed:

```sh
(cd extensions/session-migrate && go test -race -count=1 ./... && go vet ./...)
PIG_BIN=/tmp/pig-session-migrate-stock python3 -m test.integration.session_migrate
PIG_BIN=/tmp/pig-session-migrate-fused-v2 PIG_FUSED=1 python3 -m test.integration.session_migrate
PIG_BIN=/tmp/pig-session-migrate-fused-v2 PIG_FUSED=1 python3 -m test.integration.fused_runtime
nix build . --no-link
```

The Nix build uses its separately pinned PiG source (`nix/package.nix`), retains
the existing dependency hash (no new external Go dependencies), and runs the
migration integration with the built fused binary as an install check. Its
fused-runtime, savelast and pins RPC install checks also passed.

The fixture is upstream's unmodified sanitized native-produced Claude Code
2.1.209 portable-rich transcript, SHA-256
`9103243215e8cfb960495d2e0097c2c6d5787fe6dc93e2166fad7c7ebe827c3c`.
Capture/client/sanitizer provenance is retained beside it. We did not run Claude
against a live vendor or read the operator's private session files.

The integration independently proves:

- Inspection reports counts and creates no session.
- Command import writes to the host-selected directory; path with spaces works.
- Actual host switch loads the new session UUID/path and native title.
- Three historical tool calls/results and one user image survive native loading.
- Private thinking/documents are absent from the portable target.
- Content-free manifest hashes/counts are correct; new session/audit modes are 0600.
- Import performs no model requests or tool executions.
- A local HTTP dummy provider receives the imported conversation and three tool
  exchanges on continuation; PiG appends its answer without rewriting imported
  history. The continued file reopens in a fresh host process.
- `save` creates another independent session without switching the active one.
- Malformed ancestry emits an extension error without leaking its message body,
  adding files or replacing the active session.
- A synthetic Claude compact-boundary/summary imports as a native compaction;
  the real provider request includes summary + subsequent turns, **not** the
  pre-compaction history.
- Source SHA-256 is unchanged throughout.
- The seven-member fused process registers `session-migrate` without any runner
  child process. This is execution-placement evidence, not just registration.

Unit/race tests additionally cover graph-order replay (child physically before
parent), inactive forks, UUID-less legacy input, declared preserved-compaction
loops versus arbitrary cycles, duplicate/missing IDs, sidechains, mixed source
identities, orphan/duplicate/unresolved tools, exact large JSON numeric arguments,
ordered result text/images, private-thinking/document/image omissions, title
precedence, malformed UTF-8/JSON, cancellation, size bounds, failed publication,
source UUID resolution ambiguity, independent identities and non-overwrite.

## Remaining scope

Other source harnesses, exports and upstream catalog are not implemented.
Interactive TUI presentation, actual terminal graphics, a user-private Claude
transcript and vendor-backed continuation are not claimed. No credentials,
alerts, account resets or PiG core modifications were involved. Source historical
tool names are not mapped to the current executable tool set. Save/import require
host session persistence; only inspection is supported with `--no-session`.
