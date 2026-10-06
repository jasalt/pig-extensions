# Provenance and license inventory

New project work: MIT, copyright 2026 Jarkko Saltiola (root `LICENSE`).
Dependencies remain under their own licenses; the PiG Go extension SDK v0.3.1
is MIT (`github.com/MichaelKinsy/PiG/extensions/sdk`, upstream `LICENSE`).

## savelast

Behavior adapted from Atom Mac's `atomdmac/pi-savelast`, commit
`efb580c1e7f95e230c2c413021b6680abfa287c7`, package version 1.0.0.
Local reference: `../kmet/target/reference/pi-savelast/index.ts`.
SHA-256: `f5fe49f8e340936b93322236c1227024abc8bf608afda5d065e07443820d7fc3`.
Its package metadata declares **ISC**, author **Atom Mac**. That checkout has
no standalone license file or copyright year; none has been invented here.
The ISC terms applying to adapted material are retained below. Original work
is not relicensed by the root MIT license.

> Permission to use, copy, modify, and/or distribute this software for any
> purpose with or without fee is hereby granted, provided that the above
> copyright notice and this permission notice appear in all copies.
>
> THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
> WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
> MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
> ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
> WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
> ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
> OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

Approved differences: active-branch reads instead of whole-log reads;
Go/PiG SDK implementation and explicit session-read diagnostics.
No changes to overwrite, whitespace, block joining or latest-message fallback.

## notify-pushover

Adapted from user-authorized local Pi `pushover-human/index.ts` at
`../lima-default/home-manager/pi/agent/extensions/pushover-human/index.ts`.
SHA-256: `28fdb9c1914c67b08c89fc0cb0c4e06d657db90f7a403f340394f71a5a991a1f`.
No published version or standalone license is asserted for that local file;
user authorization and the selected MIT project licensing govern this adaptation.
New Go code: copyright 2026 Jarkko Saltiola, root MIT license. No kmet code copied.

Approved differences: corrected directory/name; PiG titles and `PIG_PUSHOVER_*`;
selected-agent-directory `notify-pushover.json`; 15-second timeout and redirect
refusal. Error hardening also redacts form-encoded secrets and diagnostic content;
owned sends cancel/join on shutdown. Original tool/command names, credential
precedence/cache lifecycle, form limits and one-way-only behavior are retained.

## codex-usage (qualification in progress)

Adapted from `jasalt/chatgpt-openai-api-adapter`, commit
`94f45568b4bd7842b1aef362cc3ba883b1312951`, `contrib/pi-codex-usage.ts`.
Source SHA-256: `9e6bf72c5e050b51d7825a667a351062afe6b81bb737507a9f4175e181cea3b7`.
MIT, copyright 2026 Jarkko Saltiola. Original license retained verbatim at
`extensions/codex-usage/LICENSE`, SHA-256
`b4e8cd39baa974c8d693b5f8c7078ace3a481172f539e0e42a57075d675911e7`.
No Node runtime or machine-local SDK replacement is introduced.

Differences: PiG's resolved model-auth/headers facade rather than Pi's provider
object; PiG D26 `originator: pig`; approved native notifications; owned HTTP/ticker
cancellation and stale-generation suppression. Security hardening scrubs raw and
encoded credentials from returned/displayed data and clears authentication headers
on cross-origin redirects. Reset redemption remains immediate, without confirmation
or automatic retry. Formatting currently qualified for en_US/en_GB, local timezone
and DST boundaries; no blanket locale parity is asserted.

## imgview

Adapted from Gregory Johnson's `gregjohnso/pi-imgview`, commit
`17b568e8e3b70d009adb8ac090d2b3280f065f11`; local reference
`../kmet/target/reference/pi-imgview-source`. MIT, copyright 2026 Gregory
Johnson; license retained verbatim at `extensions/imgview/LICENSE`
(SHA-256 `abd753e10250d7afb92c8b5ce361586d62d85f4d130a26e8707066d3a198dfcd`).

| Source file | SHA-256 |
| --- | --- |
| `extensions/imgview/index.ts` | `94288bb36c270075a9600a10753fdf95cc2dd396b5391c9d4a02bf61b27216b2` |
| `extensions/imgview/utils.ts` | `364f4b99398cbd2cd36b70e511eb69151ff97ad4612fd0deda5228a28768afdf` |
| `extensions/imgview/utils.test.ts` | `3c42adaa903f7fc74ba231ef2d7cf21a95627e867a7c8f419bad0bc6040a2157` |

The viewer HTML/CSS, MIME table, tool text and guidelines are carried over;
new Go code is copyright 2026 Jarkko Saltiola under MIT. kmet's imgview was
consulted only for its error-reporting and privacy decisions; no kmet code was
copied. The module imports PiG's public `tui` package (MIT, Hewlett Packard
Enterprise Development LP) for the slash-command image component.
Differences are listed in `extensions/imgview/README.md`.

## pins

Command and state semantics adapted from s4lv0's `s4lv0/pi-pins`, commit
`776217ccdce52aa0ae7794998847ff2253677250`, `extensions/pin.ts`
(SHA-256 `f32d9f61508d882ffe4e780095f5c83cda391d4f6526fa364b19523f9e6e1262`).
MIT, copyright 2026 s4lv0; license retained verbatim at `extensions/pins/LICENSE`
(SHA-256 `4bd84b22c7060ee70d38691df16a7d6daa3945bbd9573dce12cc84c99605cc64`).
Browser presentation follows kmet's `pins/ui.clj` (layout rules and footer
text, no code copied). `extensions/pins/highlight.go` adapts PiG v0.3.1
`tui/highlight.go` (MIT, Hewlett Packard Enterprise Development LP); its
notice is kept in the file header. New Go code: copyright 2026 Jarkko
Saltiola, MIT. Differences are listed in `extensions/pins/README.md`.

## schedule

Adapted from Alessandro Pungitore's `pungggi/pi-schedule` v0.4.0, commit
`ed5ea93f1a82ac7038fda34acbbfaa572472f9e3`, archive
`https://codeload.github.com/pungggi/pi-schedule/tar.gz/ed5ea93f1a82ac7038fda34acbbfaa572472f9e3`
(SHA-256 `e7ec9ec3120a99770834a857c65dde8ad62fcf2be6dfc4efb3c95791c2b4dfc4`).
MIT, copyright 2025 Alessandro Pungitore; license retained verbatim at
`extensions/schedule/LICENSE`. The skill is adapted from the original
`skills/schedule/SKILL.md` with corrected claims. Tool text, prompt contract,
limits, regexes and policy tables are carried over; new Go code is copyright
2026 Jarkko Saltiola, MIT. kmet's schedule port was consulted for its
fresh-row and reservation decisions; no kmet code was copied.

| Source file | SHA-256 |
| --- | --- |
| `src/action.ts` | `1384a7ed8979c19fabfb8ac4eea1623046a13b84b3915f3c6e71b79c365d5f7f` |
| `src/cli-prompt.ts` | `014bcaa247da75246b1213bffd4b02e6d018911f1a9de0268403e758f742987e` |
| `src/extension.ts` | `56dfc75a61478d790cb56786dad3c44e9129bf79d7805321ee3eec895ffe0e8e` |
| `src/ledger.ts` | `722be2ba8f9ca787e204ce80214168dc34ae0d30d3cb60f18d5431b0f33e41e2` |
| `src/lock.ts` | `610e21d12a5bbf2cfc3defeebd3e2f15f5c1f10e1f3f9ba60cc53c71dcb36d2e` |
| `src/policy.ts` | `dd0d283d79d95924b5a70346e4d9f57e1524d51367d0d04167c66b048c903d5a` |
| `src/privilege.ts` | `296f95663ef339f25832f88d9f467fdf278f6887833a104da22eea66030b5a1d` |
| `src/prompt.ts` | `a11c20be917dcb0af500a8b482ce93ce266212c262d55a70060ec01d0be50ecc` |
| `src/redact.ts` | `ef042a8981c783cfc462ea64e610b7116cc3d9f76112e81489856adf60953bc8` |
| `src/runner.ts` | `d14a07bbd52c8505cbe5f1e97243d708037a79565b5dae69c43679bdd0140e22` |
| `src/schedule.ts` | `f1b624e17c3dcdebac514dd18af903c3fb94679db0e61ee009af0db40b9432f5` |
| `src/store.ts` | `ad17be87babdb65a60cb2912b844e4e169b3dbe36b64dfb0773bdf424b2faccf` |
| `src/tool.ts` | `bc9f977775cb3cfdfd1a4c207bb04991888d9e08ec08c3b79232669055a42a61` |
| `src/trust.ts` | `6f2b8ea47ddc2218dedbd44353dfa545813b2777f64805a0e6d53d4500244109` |
| `src/types.ts` | `e79853526bece9dd78223d87c7d24b0c264d42abcb53b8df2e0046e672af3bc4` |
| `skills/schedule/SKILL.md` | `df237c174ec34cff37542530e39e5a49173176a0e4efcc4a2597668c76e4344e` |
| `LICENSE` | `5a4c2f72c4d2acfe7833b8160a339662f31c0930915b9b44009d42f24bdede49` |

Differences are listed in `extensions/schedule/README.md`.

## session-migrate

Go/PiG-targeted adaptation of `xhluca/session-migrate`, commit
`c23b1dbd21404f78be3b69d42ff4fb158ff52105`. MIT, copyright 2026 xhluca;
license retained verbatim at `extensions/session-migrate/LICENSE`.
New Go work: copyright 2026 Jarkko Saltiola, MIT.

| Consulted/copied upstream file | SHA-256 |
| --- | --- |
| `src/session_migrate/formats/claude.py` | `b914060468f8afd79500b12fd5f53da256190369af356e5dd015f2a226a8613f` |
| `src/session_migrate/formats/pi.py` | `d9fd3e1807a444f5818f40004f1994e6dbe2b9701f39a2c0c877ada486218f6c` |
| `LICENSE` | `f44a6129dfbf3f2bf68e333d33489361a2113ebe2804e5852eb39adcf58bd35b` |
| `tests/native_corpus/v1/sources/claude/2.1.209/portable-rich/native/73fea258-9467-4a17-877b-ef6bcd0898b7.jsonl` | `9103243215e8cfb960495d2e0097c2c6d5787fe6dc93e2166fad7c7ebe827c3c` |
| Same fixture's `provenance.json` | `92308721671f99ae2486df90be6db2ce12be6fbe68f12caeb6d02db067a2d3b3` |

The native transcript and provenance are copied unchanged into
`extensions/session-migrate/testdata/claude-native{.jsonl,.provenance.json}`.
Upstream describes this as an exact-client native trajectory, sanitized and
reloaded by Claude Code 2.1.209. We independently verify import and continuation
in PiG, not the upstream capture itself. Graph selection, preserved-compaction
back-edge recognition, omission rules, and Pi v3 self-anchored compaction behavior
are adapted; Python source is not executed or bundled.

Differences: **Claude → PiG only**, rather than the upstream 18×18 matrix; a Go
extension command rather than Python CLI/catalog/export. Use host-selected target
session directory/cwd, strict malformed/orphan/unresolved tool rejection instead
of ID repair, bounded reads, cancellation and no-replace private-file publication.
Only embedded data images move; private reasoning/configuration never moves.
The content-free manifest carries counts/source hash/initial-target hash. Full
behavior and exclusions: `extensions/session-migrate/README.md`.

This inventory covers implemented modules only. The prospective source inventory
in `plan.md` is not license clearance for unimplemented ports.
