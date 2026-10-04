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

This inventory covers implemented modules only. The prospective source inventory
in `plan.md` is not license clearance for unimplemented ports.
