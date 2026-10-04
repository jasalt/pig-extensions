# Running the extensions in-process

## How PiG places Go extensions

| How you run them | Placement |
| --- | --- |
| `pig -e ./extensions/<name>`, settings, Packages, `pig --piglet ./piglet.yaml` | **Subprocess.** PiG builds a runner and may pack several Go factories into one "runtime cell" process. |
| A **Piglet Binary** built from `piglet.yaml` | **In-process (fused).** The factories are compiled into the PiG executable and served over an in-memory connection. No extension process is started. |

Stock `pig` has no switch for in-process loading. Fusion happens only when
building a Piglet Binary (PiG has no dynamic Go plugins). The extension code
is the same in both placements, with the same registration contract and wire
messages.

## Build

All six extensions are fuse-compatible: conventional `Extension()` factories,
the current SDK module path, and none of the process-global calls PiG's fused
vet rejects (`os.Exit`, `os.Chdir`, `os.Stdout`, `log.Fatal*`, `fmt.Print*`).
`piglet.yaml` sets `build.extensionRealization: fused`, so the build fails
rather than falling back to a subprocess.

The native builder compiles PiG from source. It needs a **git checkout** of
the PiG version you are building against (it records the source revision),
named by `PIG_SOURCE_ROOT` unless you run from inside it. A `go install`ed
`pig` does not fetch its own source. Use a clean checkout or a local clone so
the binary matches a known commit:

```sh
git clone --no-checkout /path/to/PiG /tmp/pig-src
git -C /tmp/pig-src checkout 0e6ed0048282a15531ea1652a5833de4da4dd1a7   # PiG 0.3.1 reviewed here

cd /path/to/pig-extensions
pig piglet validate ./piglet.yaml
PIG_SOURCE_ROOT=/tmp/pig-src pig piglet build ./piglet.yaml --format binary --out ./pig-extensions
pig piglet show pig-extensions      # components: extension/<name>  fused  binary
```

Run the result like `pig`; the six extensions are always active in it:

```sh
./pig-extensions
```

Edit `piglet.yaml` to drop extensions you do not want. In particular, the
scheduler and the Pushover notifier are active whenever they are included.
Add `--sign-key` to sign the binary (`pig piglet keygen`). A built binary
pins its components: rebuild to pick up extension changes, and use stock
`pig -e ...` with `/reload` while developing.

## Verified

Against PiG `0e6ed00` (0.3.1), Linux amd64:

- `pig piglet show` reports all six as `fused / binary`.
- `test.integration.fused_runtime`: all commands are registered with **no**
  child process of the PiG process. Stock `pig -e` with the same sources
  shows a packed Go `runner` child.
- The full real-host integration suite passes against the fused binary
  (`PIG_BIN=<binary> PIG_FUSED=1`), with all six extensions loaded together.
  Excluded: `savelast_mutation`, which needs to load mutant source and does
  not apply to a prebuilt binary.

## Behavior that changes in-process

- **Shared process.** A crash in extension-owned code would take PiG down. The SDK recovers handler panics, and the scheduler guards its own goroutines. codex-usage's poller goroutines were not given an extra guard.
- **imgview slash-command images** use the host's live terminal
  capabilities, cell size and Kitty image IDs (in a subprocess they are
  detected from the environment). Tool-result images are host-rendered in
  both placements.
- **pins hyperlinks** follow the host's capability settings for the same reason.
- Session replacement and `/reload` still construct fresh extension
  instances; per-session state behaves as in subprocess placement.
