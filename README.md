# pig-extensions

Independent, optional Go extensions for PiG. Target checkout:
`0e6ed0048282a15531ea1652a5833de4da4dd1a7` (PiG `0.3.1+0.87.1`), Linux.
Namespace: `github.com/jasalt/pig-extensions`. New work is MIT;
[third-party notices](docs/provenance.md) remain applicable.

All six priority extensions are implemented and tested against the real host.
An additional [session-migrate](extensions/session-migrate/README.md) Go extension
imports Claude Code sessions into PiG; subprocess and fused continuation are verified.
BTW is not ported (substrate proven), and `extension-toggle` is intentionally
excluded. Open items are listed in [validation](docs/validation.md#remaining-scope).

| Extension | Current state |
| --- | --- |
| [savelast](extensions/savelast/README.md) | Implemented; unit, SDK error-boundary and real RPC/mutation tests |
| [notify-pushover](extensions/notify-pushover/README.md) | Implemented; offline HTTP, real command/tool and cancellation tests |
| [codex-usage](extensions/codex-usage/README.md) | Implemented; native OAuth/adapter caller, reset, replacement/disconnect and unit/race tests; combined/terminal gates remain |
| [pins](extensions/pins/README.md) | Implemented; unit/race, real-host RPC command/state and interactive PTY browser tests |
| [imgview](extensions/imgview/README.md) | Implemented; unit/race, real-host RPC command/tool and interactive PTY renderer tests; actual graphics and desktop browser not verified |
| [schedule](extensions/schedule/README.md) | Implemented; unit/race and real-host privilege/queued-tier/shell/trust/tool tests. Startup-wave substitute needs your acceptance |
| [session-migrate](extensions/session-migrate/README.md) | Claude → PiG importer; unit/race, native-produced Claude fixture, real-host import/reopen/continuation and fused tests |
| btw | Not ported. Child-session substrate proven ([feasibility](docs/feasibility.md#btw--child-session-substrate-proven-port-not-started)); overlay/UI work remains |

## Select exactly what you need

Each `extensions/<name>` is an independent importable Go factory/module, using
SDK v0.3.1 and Go floor 1.26. The repository root is not an extension bundle.
There is no mandatory umbrella Package or Node runtime; the root
`piglet.yaml` is an optional in-process build recipe, not a bundle.

```sh
# One-off source-root load, with ambient extensions disabled:
pig --no-extensions -e /absolute/path/to/pig-extensions/extensions/savelast

# Build/register only, without installing:
pig install --validate-only --json ./extensions/savelast
```

Validation can execute constructors; these constructors do not send notifications,
launch browsers, mutate credentials, or schedule work. No global installation,
remote publication or PiG core changes are made by the project tests.
For installed resource selection use **`pig config`** or **`pig config --local`**;
there is deliberately no `extension-toggle` port or new settings plane.

## Run them in-process

Stock `pig` runs Go extensions as subprocesses. To run all seven inside PiG's
own process, build a fused Piglet Binary from [`piglet.yaml`](piglet.yaml):

```sh
PIG_SOURCE_ROOT=/path/to/PiG-checkout pig piglet build ./piglet.yaml --format binary --out ./pig-extensions
```

See [docs/in-process.md](docs/in-process.md) for requirements, verification
and what changes in-process.

### Nix / Home Manager

`nix build .` builds the same recipe with pinned PiG source and Go dependencies;
no preinstalled PiG or sibling checkout is needed. The result provides `pig` and
`pig-extensions`. The flake also exports `homeManagerModules.default`.
See [docs/nix.md](docs/nix.md) for remote builds, Home Manager integration,
local overrides, and runtime setup.

## Release binaries

Pushing a `v*` tag triggers GitHub Actions to build the fused Linux x86-64
executable through Nix and publish a tar archive, license notices, and SHA-256
checksum to a GitHub Release. Downloaded binaries do not require Nix or Go.
See [docs/releases.md](docs/releases.md) for publishing, installation, and
verification scope.

## Upstream issues

Problems found in PiG and in the original Pi extensions, each with an
isolated reproduction, are in [UPSTREAM-ISSUES.md](UPSTREAM-ISSUES.md)
(not filed upstream).

## Verification

[Validation evidence](docs/validation.md) records exact scope and known remaining
work. Tests isolate agent/home/project/session state and use dummy credentials.
Pushover integration uses a local TLS proxy and a hermetic model, not live delivery.
Never use normal tests as approval for real alerts, reset redemption or other
account mutations. Desktop/terminal-graphics success is not implied by text tests.

The complete contract is [plan.md](plan.md). `pi-handoff.md` is the initial
historical snapshot; consult current files and validation evidence before relying
on it. Changes are committed atomically as subsequently authorized by the user;
no remote is created or publication performed.
