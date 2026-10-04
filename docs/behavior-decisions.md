# Implemented behavior decisions

The complete, user-ratified decision table and remaining gates are in
[`plan.md`](../plan.md#8-recorded-user-decisions). This document records differences
actually implemented, rather than declaring planned behavior accepted.

## savelast

- Read the latest assistant on the **active branch**, not the whole entry log.
- Never select an older text-bearing response when the latest is textless.
- Retain original missing-versus-null content behavior, text block joining,
  ECMAScript blank detection, exact written bytes and overwrite without prompting.
- PiG session read failures are notified once and do not become a second generic
  slash-command error. A source-baseline difference exists only in branch selection
  and host/API/error adaptation; no intended whitespace/output change.

## notify-pushover

- Correct the extension directory/name, but retain `notify_human` and
  `/notify-human-test` identities.
- Use PiG titles, `PIG_PUSHOVER_*` and the selected agent directory's
  `notify-pushover.json`. No legacy aliases, migration or credential mixing.
- Retain original whole-config precedence, late loading and new-session refresh.
- Retain the approved 15-second timeout and refusal to follow redirects.
- Keep native keyed status and notifications; do not persist notices as model
  messages or replace the footer.
- Preserve original UTF-16 field slicing and form encoding. Credential-redaction
  hardening includes form-encoded values, read failures and status phrases.
- Own request cancellation, shutdown joins and idle HTTP cleanup. A send error
  does not mean the remote service did not accept the notification.

## imgview

- Tool images are ordinary ordered PiG tool-result content; the host renders
  them with its own capabilities and settings.
- Slash-command images use PiG's public `tui.Image`. In subprocess placement
  its terminal capabilities come from the inherited environment, because the
  Go SDK does not expose the host's resolved capabilities; host
  `terminal.*` overrides and measured cell sizes are not applied there. Fused
  placement shares the host's state. Recorded as a known difference rather
  than inventing a terminal protocol.
- Honest browser errors (no "opened" claim on failure; browser-only failure is
  an error), private viewer files kept after unload, and a pre-transport
  refusal for images that cannot fit one extension frame.
- Linux `xdg-open` only.

## Not accepted by these decisions

No change to scheduler policy/startup suppression, rendering requirements,
Codex reset semantics, BTW modes or tool isolation has been approved through
implementation convenience. No live alerts, reset redemption, terminal graphics,
desktop opener, packed-cell reload or conditional child-agent capability is
claimed from the two ports' offline RPC tests.
