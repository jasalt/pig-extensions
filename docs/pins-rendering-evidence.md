# Pins Go rendering probe — historical feasibility evidence

> **Outcome (2026-10-04):** the pins extension now uses a per-instance copy of
> PiG's lexer-to-theme mapping fed by the host theme snapshot, so the
> highlighting counterexample below no longer applies to it; see
> `extensions/pins/README.md` and `docs/validation.md`. The probe still
> demonstrates the underlying public-API facts. Hyperlink-setting propagation
> and height-only overlay invalidation remain open SDK gaps.

Target: PiG `0e6ed0048282a15531ea1652a5833de4da4dd1a7`, SDK/root module v0.3.1,
Go 1.27.1, Linux. No PiG/SDK edits or Node runtime were introduced.

## Reproduce

```sh
(cd test/fixtures/pins-render-probe && GOTOOLCHAIN=go1.27.1 go test ./...)
(cd test/fixtures/pins-render-probe && GOTOOLCHAIN=go1.27.1 go test -race ./...)
(cd test/fixtures/pins-render-probe && GOTOOLCHAIN=go1.27.1 go vet ./...)
python3 -m test.integration.pins_render_probe
python3 -m test.integration.pins_render_pty
```

`pins_render_probe` proves RPC is not accepted as a terminal browser even though
HasUI may be true. Its default RPC palette/capabilities are not a custom-theme
terminal proof. The actual PTY probe uses a private temporary agent/project/session,
explicit custom theme and `terminal.hyperlinks=false`; ambient credentials and
shared Pi-directory mode are disabled. No model/provider, browser or alert is used.

## Production-path counterexamples

The actual host resolves `probe-custom` with `syntaxKeyword=#123abc`:

```text
hostKeyword = ESC[38;2;18;58;188m
native child highlighting = ESC[38;2;86;156;214mfunc ...
```

The public `tui.HighlightCode` reads process-global `ActiveTheme()`, not the supplied
per-widget SDK theme. Native `MarkdownTheme` callbacks correctly style headings
and ordinary text, but forwarding `HighlightCode: tui.HighlightCode` retains the
child's default syntax colors. This is not accepted custom-theme parity.

The parent explicitly disables hyperlinks. With a simulated Kitty environment,
the child library reports `Hyperlinks:true` and native Markdown emits:

```text
ESC]8;;https://example.invalid/pin ESC\\ ... ESC]8;; ESC\\
```

The host shared `StatePayload` already carries resolved terminal capabilities, but
Go SDK `Extension.handleNotify(state_update)` discards that payload. Native Markdown
link assembly uses global `GetCapabilities()`. Child environment detection cannot
reproduce parent settings overrides. Mutating shared globals, assuming subprocess
isolation or switching to an invented renderer is not an accepted fix.

## Geometry/input evidence and remaining red gate

Actual `Context.Custom` displays the borderless probe; `G` scrolls its owned content
and `q` closes it. Native header, code and table lines are captured. At observed
sizes 100x30, 32x12 and 8x4, the component emits full-width lines and 80%-height
viewports (floor, minimum one row). These are component reports plus a real PTY
mount/input path, not exact final screen-cell/host-position comparisons.

An initial test comparing report height with rows failed at a 1x1 resize: height
changed during rendering. The report now records both the single height snapshot
used for layout and the latest SDK height, avoiding a false assertion caused by
sampling twice. Tiny/current-height frames can still be missing or coalesced when
width and height notifications race. SDK `height_change` updates cached height but
does not request overlay redraw, unlike `width_change`. Full resize is **unaccepted**.
The feasibility test explicitly retains these counterexamples; it is not a weakened
passing pins acceptance test.

Inputs/settings, raw terminal bytes, native Markdown/host-theme facts and component
frames are saved in private unique `pins-render-evidence-*` directories. Representative
runs: `/tmp/pins-render-evidence-_k2wocmp` (missing narrow/current-height frames) and
`/tmp/pins-render-evidence-4ly34mqb` (all observed sizes). These are generated test
artifacts, not portable committed fixtures; rerun to obtain authoritative fresh data.
No Kitty/iTerm2 graphics, desktop opener, full tree/resume/reload browser or exact
terminal-screen equivalence is claimed.

## Separate API work proposal — approval required

A qualified native route needs:

1. Retain/expose immutable host-resolved terminal capabilities in the Go SDK.
2. Native Markdown rendering with per-instance capabilities (not process-global
   detection/override), and a public per-instance theme/style highlighter entrypoint.
3. Overlay invalidation for height changes, with resize ordering/snapshot tests.

These are proposed changes to the target host/public SDK/rendering API, not implicitly
approved extension work. Implement the required applicable SDK/conformance/parity
checks only after approval and an agreed target update. Alternative local highlighter
or renderer adaptations need an explicit reviewed design; do not quietly replace
native behavior or adopt Node. Pin model/state logic can proceed independently while
the UI route remains blocked.

## Provenance

The test fixture and driver are MIT, copyright 2026 Jarkko Saltiola. They use PiG's
public SDK/TUI packages (MIT, Copyright Hewlett Packard Enterprise Development LP)
and derive a temporary custom theme from the PiG v0.3.1 module copy of `tui/theme_dark.json`; dependency
licenses remain applicable. No third-party renderer code or demo asset is copied.
Original pin source reviewed: `s4lv0/pi-pins` commit
`776217ccdce52aa0ae7794998847ff2253677250`, source SHA-256
`f32d9f61508d882ffe4e780095f5c83cda391d4f6526fa364b19523f9e6e1262`.
Kmet's `pins/ui.clj` is only the selected borderless presentation reference.
