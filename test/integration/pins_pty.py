"""Interactive /pin browser through the real host TUI and a custom theme.

Run: python3 -m test.integration.pins_pty. Proves overlay mount, host-theme
syntax colors, Markdown structure, scrolling, pin switching, closing and focus
return on the real terminal stream. Exact screen-cell composition is not
compared.
"""
import json
from pathlib import Path
import re
import subprocess
import tempfile

from .pins_rpc import EXT, state
from .pty_host import PTYHost
from .rpc import ROOT, seed

KEYWORD = "#123abc"
KEYWORD_SGR = b"\x1b[38;2;18;58;188m"


def dark_theme():
    module = subprocess.run(["go", "list", "-m", "-f", "{{.Dir}}", "github.com/MichaelKinsy/PiG"],
                            cwd=EXT, check=True, capture_output=True, text=True).stdout.strip()
    return json.loads((Path(module) / "tui/theme_dark.json").read_text())


def run():
    long_tail = "\n\n".join(f"Tail line {i}" for i in range(40))
    first = ("# Native heading\n\n```go\nfunc main() {\n\treturn\n}\n```\n\n| A | B |\n|---|---|\n| yes | no |\n\n"
             + long_tail)
    pins = [dict(id=1, label="Markdown pin", text=first, pinnedAt=1),
            dict(id=7, label="Second pin", text="Beta only", pinnedAt=2)]
    with tempfile.TemporaryDirectory(prefix="pins-pty-") as temp:
        cwd = Path(temp)
        theme = dark_theme()
        theme["name"] = "pins-custom"
        theme["colors"]["syntaxKeyword"] = KEYWORD
        theme_file = cwd / "pins-custom.json"
        theme_file.write_text(json.dumps(theme))
        session = cwd / "session.jsonl"
        seed(session, cwd, [state("s1", None, dict(pins=pins, nextId=8))])
        with PTYHost(cwd, [EXT], session, rows=30, cols=100,
                     settings=dict(theme="pins-custom", terminal=dict(hyperlinks=False, trueColor=True)),
                     args=["--theme", str(theme_file), "--use-theme", "pins-custom"]) as host:
            host.wait_command("pin", "Pin assistant messages and recall")
            mark = host.command("/pin show")
            host.pump_until(lambda: "📌 2 pins".encode() in host.raw[mark:])
            host.settle()
            frame = bytes(host.raw[mark:])
            assert KEYWORD_SGR + b"func" in frame or KEYWORD_SGR + b"return" in frame, "host theme keyword color missing"
            assert b"# Native heading" not in frame and b"Native heading" in frame
            assert "│".encode() in frame and b"yes" in frame, "table structure missing"
            assert "❯ #1 · Markdown pin".encode() in frame and "  #7 · Second pin".encode() in frame
            assert re.search(r"#1 · 1–\d+/\d+".encode(), frame), "position line missing"

            mark = len(host.raw)
            host.write("G")
            host.pump_until(lambda: b"Tail line 39" in host.raw[mark:])
            mark = len(host.raw)
            host.write("\x1b[6~")
            host.pump_until(lambda: b"Beta only" in host.raw[mark:] and "#7 · all visible".encode() in host.raw[mark:])
            mark = len(host.raw)
            host.write("\x1b[5~")
            host.pump_until(lambda: "❯ #1 · Markdown pin".encode() in host.raw[mark:])

            # Width change re-renders the overlay at the new width.
            mark = len(host.raw)
            host.resize(30, 60)
            host.pump_until(lambda: "📌 2 pins".encode() in host.raw[mark:])
            host.settle()

            host.write("q")
            host.settle()
            # Focus returned to the editor: the next command runs.
            mark = host.command("/pin help")
            host.pump_until(lambda: b"Pin the latest nonempty assistant text" in host.raw[mark:])

            # Selection by number, then close with Escape.
            mark = host.command("/pin list 7")
            host.pump_until(lambda: "❯ #7 · Second pin".encode() in host.raw[mark:])
            host.write("\x1b")
            host.settle()
            mark = host.command("/pin rm 7")
            host.pump_until(lambda: 'Removed pin #7 "Second pin"'.encode() in host.raw[mark:])
            assert host.entries("pin-state")[-1]["data"]["pins"][0]["id"] == 1
    print("PASS /pin interactive browser: host-theme highlighting, markdown/table, G/PgDn/PgUp, width re-render, q/Esc close, focus return")


if __name__ == "__main__":
    run()
