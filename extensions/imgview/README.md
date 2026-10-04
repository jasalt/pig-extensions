# imgview

Shows existing images from inside PiG: inline in the terminal, in the system
browser, or both. Go port of [gregjohnso/pi-imgview](https://github.com/gregjohnso/pi-imgview)
at `17b568e8e3b70d009adb8ac090d2b3280f065f11`. Upstream's MIT license is kept
in [LICENSE](LICENSE).

```sh
pig --no-extensions -e /absolute/path/to/pig-extensions/extensions/imgview
pig install --validate-only --json ./extensions/imgview   # build/register only
```

## Tool: `show_image`

| Parameter | Meaning |
| --- | --- |
| `source` (required) | Local path (absolute, relative to the current cwd, or `~/...`), `http(s)://` URL, or `data:` URI |
| `mode` | `terminal` (default), `browser` or `both` |
| `caption` | Optional note, up to 200 characters |

Terminal and both results are ordered text-then-image PiG tool results, so the
host renders the image with its own terminal capabilities and image settings,
and models that accept images receive it. Browser-only results contain text
only. Prompt guidelines forbid choosing `browser`/`both` unless the user asks.

## Commands

| Command | Effect |
| --- | --- |
| `/imgcat <source>` | Inline image in the transcript |
| `/imgshow <source>` | Open in the default browser |
| `/imgboth <source>` | Both |

The whole trimmed argument is the source, so paths with spaces need no quotes.
`/imgcat` and `/imgboth` add a persisted `imgview-image` message without
starting a turn (`followUp`, `triggerTurn: false`). Its text summary enters
model context; the image bytes stay in the message details.

## Loading

- MIME comes from magic bytes first (PNG, JPEG, GIF, WebP, BMP, AVIF, SVG),
  then the file/URL extension, then a declared data URI or HTTP type.
  Anything other than those seven types is rejected.
- Data URIs accept base64 (Node-compatible: URL-safe, unpadded, whitespace
  ignored) or percent-encoded payloads, where `+` stays literal.
- HTTP(S) follows redirects, honors proxy environment variables and is
  cancelled with the tool call. Files and downloads are read fully into memory.
- Images above 8 MiB add a warning note only.
- An image whose base64 cannot fit in one PiG extension message (128 MiB
  frame, 1 MiB reserved) is refused for terminal mode before sending; `both`
  keeps the browser view and states that the inline image was omitted.

## Browser viewer

A self-contained HTML viewer (image embedded, label escaped) is written to
`$TMPDIR/pig-imgview/imgview-<ms>-<random>.html`, directory mode `0700`,
file mode `0600`, and opened with `xdg-open <viewer>` (argv, no shell, own
session). Viewers stay after unload so an open tab keeps working; delete old
ones manually. A missing opener is an error: browser-only tool calls fail,
`both` keeps the inline image and reports the browser error, and no message
claims that a viewer was opened. A started opener cannot confirm that a
browser window appeared.

## Behavior differences from upstream

- PiG naming (`pig-imgview` temp directory, "PiG" in the model hint).
- Linux only (`xdg-open`); macOS/Windows openers are not ported.
- Launch errors are reported honestly (upstream could report "opened" and
  "failed" together, and browser-only failures were not errors).
- Transport-size refusal described above.
- **Slash-command rendering in the default subprocess placement:** the
  renderer uses PiG's public `tui.Image`. PiG's Go SDK does not pass the host's
  resolved terminal capabilities to extensions, so a subprocess extension
  detects them from the inherited environment (`TERM`, `TERM_PROGRAM`,
  `KITTY_WINDOW_ID`, `PI_IMAGE_PROTOCOL`, ...) and uses default cell sizes.
  Host `terminal.*` settings overrides and measured cell sizes do not reach it.
  In a fused Piglet Binary the extension runs in the host process and shares
  the host's capabilities. Tool-result images are always host-rendered.

## Tests

```sh
(cd extensions/imgview && go test ./... && go test -race ./... && go vet ./...)
python3 -m test.integration.imgview      # real host, RPC + hermetic model
python3 -m test.integration.imgview_pty  # real host, interactive PTY renderer
```

A fake `xdg-open` records argv; no browser starts. PTY output proves protocol
bytes or fallback text reached the terminal stream, not that a graphical
terminal displayed the image.
