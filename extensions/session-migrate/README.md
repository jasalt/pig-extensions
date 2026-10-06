# session-migrate

Go/PiG-targeted adaptation of [xhluca/session-migrate](https://github.com/xhluca/session-migrate).
**Implemented route: Claude Code → PiG.** This is not a port of upstream's
18×18 adapter matrix, catalog database, exporters, or Python CLI. No Python/Node
runtime is needed by the extension. MIT; upstream license is retained in `LICENSE`.

## Use

```sh
pig --no-extensions -e /absolute/path/to/extensions/session-migrate
```

Inside PiG:

```text
/session-migrate inspect claude /path/to/SESSION.jsonl
/session-migrate import claude /path/to/SESSION.jsonl
/session-migrate save claude /path/to/SESSION.jsonl
/session-migrate import claude SESSION_UUID
```

- `inspect`: validate and report content-free preservation/omission counts; no writes.
- `import`: write a new session and manifest, then await the host's session switch.
- `save`: write the same files without switching; resume later with `pig --session PATH`.
- Paths are relative to the host cwd; `~/` and paths with spaces are accepted.
  The entire remainder is the literal path, optionally surrounded by matching
  quotes. It is not shell syntax; there are no shell expansions or trailing flags.
- UUID lookup checks `CLAUDE_CONFIG_DIR/projects/*/UUID.jsonl`, defaulting to
  `~/.claude/projects`. Ambiguous matches fail; supply the exact path instead.
- `save`/`import` require an idle host with session persistence enabled
  (not `--no-session`). Inspection also works without persistence.

The target uses **the host's selected session directory and current cwd**, not
Claude's source cwd and not a guessed `~/.pig` path. Each import has a new UUID,
a new linear tree, and its native title. The source and the outgoing session are
not changed. A cancelled/failed switch retains the saved file and announces its
path. Import never invokes a model or executes recorded tools.

## Portable data and safety

- Select the active UUID ancestry (latest `last-prompt.leafUuid`, otherwise last
  eligible conversation record); do not flatten forks or subagents. Graph order
  handles tool-result children physically appended before their parents. Recover
  uniquely matched result-only children outside that ancestry only when both
  `parentUuid` and `sourceToolAssistantUUID` point to the selected call's assistant
  record and the nonempty session ID matches. Insert them after that assistant;
  do not recover sibling text, sidechains, metadata, or already-resolved calls.
  Ambiguous eligible results fail rather than choosing a sibling.
- Preserve user/assistant text, tool-call names/IDs/object arguments and ordered
  tool results including `isError`. Tool names are historical: they are not
  remapped to PiG tools or installed as capabilities.
- Preserve embedded base64 PNG/JPEG/GIF/WebP user/tool-result image blocks.
  URL images are not fetched. Image transport, not graphics display, is tested.
- Preserve compact-summary text with a native self-anchored v3 compaction entry:
  pre-summary entries remain in history but are excluded from provider context.
  Claude's metadata-declared preserved-segment back edge is recognized;
  undeclared cycles fail.
- Drop private/signed/redacted thinking, documents, unknown blocks, inactive
  branches and source-specific metadata. No hooks, credentials, policies, MCP,
  system prompts, or runtime configuration are imported.
- Reject missing/duplicate graph IDs, cycles, mixed active session identities,
  orphan/duplicate/unresolved tool calls/results, sidechain-only transcripts,
  malformed JSON/UTF-8 and empty portable context. This is stricter than upstream's
  tool-ID repair behavior: incomplete exchanges are not silently synthesized.
- Regular file input: 256 MiB total, 32 MiB per JSONL record. All source reads and
  writes finish inside the handler with cancellation checks; no detached work.
- New files are mode 0600, staged/fsynced and published with no-replace hard links.
  Ordinary publication errors roll back the pair. The pair is not a crash-atomic
  transaction: a crash between links can leave an audit-only sidecar or hidden
  staging file. No existing target is overwritten.

`PATH.jsonl.migration.json` contains source SHA-256, record/count accounting,
new session ID and **initial** target SHA-256, not conversation text or title.
Opening/continuing the session can append native model/thinking metadata and
messages, so the initial hash is not a live checksum. Omissions are counted by
records/blocks, not by bytes/tokens; this is not a forensic copy.

## Fusion

The root `piglet.yaml` includes this conventional `Extension() *sdk.Extension`
factory with required fused realization. It can also be selected independently:

```yaml
name: migration
extensions:
  - name: session-migrate
    origins: [local:./extensions/session-migrate]
build:
  extensionRealization: fused
```

Build with an absolute PiG source root:

```sh
PIG_SOURCE_ROOT=/path/to/PiG pig piglet build ./piglet.yaml --format binary --out ./pig-extensions
```

## Verification

```sh
(cd extensions/session-migrate && go test -race ./... && go vet ./...)
python3 -m test.integration.session_migrate
PIG_BIN=/path/to/fused-binary PIG_FUSED=1 python3 -m test.integration.session_migrate
```

Verified against the repository's pinned PiG `0e6ed004...` (`0.3.1+0.87.1`),
Linux amd64, both subprocess and fused (all seven recipe members). The integration
uses upstream's sanitized **native-produced Claude Code 2.1.209** transcript,
not a claimed live import of your private session. The upstream capture provenance
is retained beside the fixture. It checks title, messages, three tool exchanges,
an image, hashes/modes, source immutability, successful host switching, hermetic
provider continuation, restart/reopen and save-without-switch. A synthetic
compaction checks the actual provider context; malformed ancestry must leave
both the active session and disk inventory unchanged. A separate runtime check
confirms fusion starts no extension subprocess.

No live vendor credentials, real alerts, or account mutations are used. Interactive
TUI presentation and other harness formats are not verified or implemented.
