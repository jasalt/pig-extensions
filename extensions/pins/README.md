# pins

Pin assistant messages and recall them in a dismissible overlay browser,
without taking permanent screen space. Go port of
[s4lv0/pi-pins](https://github.com/s4lv0/pi-pins) at
`776217ccdce52aa0ae7794998847ff2253677250`, with kmet's borderless browser
presentation. Upstream's MIT license is kept in [LICENSE](LICENSE).

```sh
pig --no-extensions -e /absolute/path/to/pig-extensions/extensions/pins
pig install --validate-only --json ./extensions/pins   # build/register only
```

## Commands

| Command | Effect |
| --- | --- |
| `/pin [label]` | Pin the latest nonempty assistant text (free label, auto-generated if omitted) |
| `/pin pick` | Pick one of the ten latest assistant texts |
| `/pin show [n]` | Open the browser, optionally at `#n` |
| `/pin list [n]` | Alias for `show` |
| `/pin rm <n>` | Remove pin `#n` |
| `/pin clear` | Remove all pins (IDs restart at 1) |
| `/pin help` | Show the instructions |

Subcommands are case-insensitive and tab-complete. Any other text is a label.
A first word that names a no-argument subcommand (`pick`, `clear`, `help`)
followed by more text is also a label, so `/pin clear plans` pins with the
label "clear plans". `list` is an alias only when followed by nothing or a pin
number, so upstream's documented `/pin list of plugins` remains a label.

## Browser

Full terminal width, no border or side padding, 80% of the terminal height,
anchored at the top. A header, a windowed pin list, the selected pin rendered
with PiG's native Markdown component, and a key/position footer. The pins are
an immutable snapshot taken when the browser opens.

| Key | Action |
| --- | --- |
| `↑` / `↓` | Scroll content |
| `PgUp` / `PgDn` | Previous / next pin |
| `g` / `G` | Top / bottom |
| `q` / `Esc` / `Enter` | Close |

The browser needs PiG's interactive terminal. In RPC, JSON or print mode
`/pin show` warns instead; the other commands work everywhere (`/pick` uses
the host's select dialog, which RPC clients can answer).

## Storage

Pins are `pin-state` custom session entries
(`{"pins":[{"id","label","text","pinnedAt"}],"nextId"}`, upstream's shape).
They never enter model context. Every command reads the last snapshot on the
**active branch**, so pins follow `/tree` navigation, survive `/reload`,
restarts and `/resume`, and a new branch starts from the pins at its fork
point. A corrupt last snapshot is reported (`pins: Invalid saved pin-state;
no pins were changed`) and is never overwritten, except by an explicit
`/pin clear`. A missing or stale `nextId` is repaired above the highest ID.

Only assistant text is pinned; thinking, tool calls and images are excluded.

## Behavior differences from upstream

- `/pin help` always shows the help text (upstream opened the browser when
  pins existed).
- `list` alias (see above), and the browser warns outside the interactive
  terminal instead of silently returning.
- State is read from the active branch on every command (upstream cached it
  at session start), and corrupt snapshots fail visibly instead of being
  treated as empty. Pin numbers must be positive decimal integers.
- kmet presentation: borderless full-width browser, list rows without time.
- Code blocks are highlighted by a per-instance copy of PiG's lexer-to-theme
  mapping that reads the host theme snapshot. PiG's public
  `tui.HighlightCode` uses the extension process's global theme, which is not
  the host theme in subprocess placement.
- Markdown hyperlink rendering follows PiG's capability detection in the
  extension process (environment-based in subprocess placement); host
  `terminal.hyperlinks` settings overrides do not reach it.
- A height-only terminal resize is not redrawn until the next key or width
  change (the Go SDK does not invalidate overlays on height changes).
- The browser is not force-closed on session events; the host tears it down
  on session replacement, reload and shutdown.

## Tests

```sh
(cd extensions/pins && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.pins_rpc   # real host commands/state over RPC
python3 -m test.integration.pins_pty   # real host interactive browser
```
