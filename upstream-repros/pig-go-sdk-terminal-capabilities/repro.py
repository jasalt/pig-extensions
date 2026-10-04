"""Host terminal settings disable hyperlinks and images; the Go extension
still sees the environment-detected values. PIG_BIN=... python3 repro.py"""
import json
from pathlib import Path
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import PTY, result  # noqa: E402

HERE = Path(__file__).resolve().parent

with tempfile.TemporaryDirectory() as home:
    agent = Path(home) / "agent"
    agent.mkdir()
    (agent / "settings.json").write_text(json.dumps(dict(
        quietStartup=True, terminal=dict(hyperlinks=False, images=False))))
    # Kitty environment: auto-detection says hyperlinks + kitty images.
    with PTY(home, [HERE], env=dict(TERM="xterm-kitty", TERM_PROGRAM="kitty")) as term:
        term.until(lambda: b"\x1b[?2004h" in term.raw)
        term.wait_command("caps-probe", "report terminal capabilities")
        term.write("/caps-probe\r")
        term.until(lambda: b"extension sees" in term.raw, 30)
        term.settle()
        line = term.raw[term.raw.find(b"extension sees"):].split(b"\x1b")[0].decode(errors="replace")
        result("pig-go-sdk-terminal-capabilities", "hyperlinks=true" in line and 'images="kitty"' in line,
               f"settings terminal.hyperlinks=false, terminal.images=false; {line.strip()}")
