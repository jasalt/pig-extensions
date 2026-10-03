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

This inventory covers implemented modules only. The prospective source inventory
in `plan.md` is not license clearance for unimplemented ports.
