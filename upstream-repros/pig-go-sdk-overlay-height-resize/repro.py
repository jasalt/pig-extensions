"""A height-only resize does not re-render an open Go overlay; a width
change does. PIG_BIN=... python3 repro.py"""
from pathlib import Path
import re
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import PTY, result  # noqa: E402

HERE = Path(__file__).resolve().parent
FRAME = re.compile(rb"PROBE frame=(\d+) width=(\d+) height=(\d+)")


def frames(raw):
    return [tuple(int(x) for x in m.groups()) for m in FRAME.finditer(raw)]


with tempfile.TemporaryDirectory() as home:
    with PTY(home, [HERE], rows=30, cols=100) as term:
        term.until(lambda: b"\x1b[?2004h" in term.raw)
        term.wait_command("overlay-probe", "open a height-reporting overlay")
        term.write("/overlay-probe\r")
        term.until(lambda: bool(frames(term.raw)), 30)
        term.settle()
        before = frames(term.raw)[-1]
        mark = len(term.raw)
        term.resize(20, 100)  # height only
        term.settle(1.5)
        after_height = frames(term.raw[mark:])
        mark = len(term.raw)
        term.resize(20, 90)  # width change
        term.settle(1.5)
        after_width = frames(term.raw[mark:])
        term.write("q")
        # The host may repaint the cached frame; what matters is that the
        # component is not asked to render again (frame counter unchanged).
        stale = all(f[0] == before[0] and f[2] == 30 for f in after_height)
        fresh = any(f[0] > before[0] and f[2] == 20 for f in after_width)
        result("pig-go-sdk-overlay-height-resize", stale and fresh,
               f"before={before}; frames after 30→20 rows: {after_height or 'none'}; "
               f"frames after 100→90 cols: {after_width}")
