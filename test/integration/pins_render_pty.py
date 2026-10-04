"""Real PTY overlay/theme/capability probe; no graphics or full pins claim."""
import fcntl
import json
import math
import os
from pathlib import Path
import pty
import select
import struct
import subprocess
import tempfile
import termios
import time

from .pins_pty import dark_theme
from .rpc import BIN, ROOT, seed

FIXTURE = ROOT / "test/fixtures/pins-render-probe"


def run_pty():
    evidence = Path(tempfile.mkdtemp(prefix="pins-render-evidence-"))
    with tempfile.TemporaryDirectory(prefix="pins-render-pty-") as temp:
        cwd = Path(temp)
        agent = cwd / "agent"
        agent.mkdir()
        theme = dark_theme()  # reviewed v0.3.1 module copy, not the moving sibling checkout
        theme["name"] = "probe-custom"
        theme["colors"]["syntaxKeyword"] = "#123abc"
        theme_file = cwd / "probe-custom.json"
        theme_file.write_text(json.dumps(theme))
        (agent / "settings.json").write_text(json.dumps(dict(
            terminal=dict(hyperlinks=False, trueColor=True), theme="probe-custom", quietStartup=True)))
        session = cwd / "session.jsonl"
        seed(session, cwd, [])
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 30, 100, 0, 0))
        env = dict(os.environ, PIG_HOME=str(cwd / "home"), PIG_CODING_AGENT_DIR=str(agent),
                   PI_CODING_AGENT_DIR=str(agent), PIG_USE_PI_DIRS="0", PIG_OFFLINE="1", PI_OFFLINE="1",
                   PI_SKIP_VERSION_CHECK="1", TERM="xterm-kitty", TERM_PROGRAM="kitty", COLORTERM="truecolor",
                   HERDR_ENV="0", TMUX="")
        for key in list(env):
            if key.endswith(("API_KEY", "ACCESS_TOKEN")) or "PUSHOVER" in key:
                del env[key]

        def controlling_terminal():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)

        proc = subprocess.Popen([str(BIN), "--no-extensions", "--no-skills", "--no-prompt-templates",
                                 "--no-context-files", "--no-themes", "--session", str(session),
                                 "--theme", str(theme_file), "--use-theme", "probe-custom", "-e", str(FIXTURE)],
                                cwd=cwd, env=env, stdin=slave, stdout=slave, stderr=slave,
                                preexec_fn=controlling_terminal)
        os.close(slave)
        raw = bytearray()

        def entries(kind):
            try:
                rows = [json.loads(line) for line in session.read_text().splitlines()]
            except (ValueError, FileNotFoundError):
                return []
            return [r["data"] for r in rows if r.get("customType") == kind]

        def pump_until(predicate, timeout=30):
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                if predicate():
                    return
                if proc.poll() is not None:
                    raise AssertionError("PTY host exited: " + raw.decode(errors="replace"))
                readable, _, _ = select.select([master], [], [], .05)
                if readable:
                    try:
                        data = os.read(master, 65536)
                    except OSError:
                        data = b""
                    raw.extend(data)
            raise AssertionError("PTY deadline: " + raw.decode(errors="replace"))

        try:
            pump_until(lambda: b"\x1b[?2004h" in raw)
            os.write(master, b"/render-probe-facts\r")
            pump_until(lambda: bool(entries("pins-render-probe-facts")))
            facts = entries("pins-render-probe-facts")[-1]
            (evidence / "facts.json").write_text(json.dumps(facts, ensure_ascii=False, indent=2))
            assert facts["hostTheme"] == "probe-custom", facts
            assert facts["hostKeyword"] == "\x1b[38;2;18;58;188m", facts
            assert facts["hostKeyword"] not in facts["childHighlight"], "native global highlighter unexpectedly honors local theme"
            assert facts["childCapabilities"]["hyperlinks"] is True, facts
            assert any("\x1b]8;;https://example.invalid/pin" in line for line in facts["markdown"]), facts
            print("RED PROOF: host custom keyword color is ignored by the native public highlighter; parent hyperlinks=false is ignored by child native Markdown.")
            before = len(raw)
            os.write(master, b"/render-probe\r")
            pump_until(lambda: b"PIN RENDER PROBE" in raw[before:])
            os.write(master, b"G")
            # The input must reach the owned component and trigger its footer.
            pump_until(lambda: b"probe " in raw[before:])
            for height, width in [(12, 32), (4, 8), (1, 1), (30, 100)]:
                fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
                # Synchronize on terminal output after the resize, not an
                # arbitrary sleep; missing height invalidation is recorded.
                readable, _, _ = select.select([master], [], [], .35)
                if readable:
                    raw.extend(os.read(master, 65536))
            os.write(master, b"q")
            pump_until(lambda: bool(entries("pins-render-probe-frames")))
            frames = entries("pins-render-probe-frames")[-1]
            (evidence / "frames.json").write_text(json.dumps(frames, ensure_ascii=False, indent=2))
            dimensions = {(f["terminalHeight"], f["width"]) for f in frames}
            assert (30, 100) in dimensions, dimensions
            # This is an expected-gap feasibility proof, not a passing browser
            # resize acceptance test. Retain every observed size, including
            # coalesced/missing frames, rather than concealing the failing gate.
            print("Observed render dimensions:", sorted(dimensions))
            assert all(f["rows"] == max(1, math.floor(f["terminalHeight"] * .8)) for f in frames), frames
            assert any(f["scroll"] > 0 for f in frames), "input before/after paint did not scroll"
            tiny_current = any(f["terminalHeight"] == 1 and f["width"] == 1 for f in frames)
            height_race = any(f["terminalHeight"] != f["latestHeight"] for f in frames)
            print("PASS actual PTY focus/scroll/close and source-frame capture; normal native header/code/table frame recorded")
            print("Tiny current-height frame:", tiny_current, "; height-update-during-render:", height_race)
            print("Full resize and renderer route remain UNACCEPTED; this probe retains the counterexamples.")
        finally:
            (evidence / "terminal.ansi").write_bytes(raw)
            (evidence / "inputs.json").write_text(json.dumps(dict(initial=[30, 100], resized=[[12, 32], [4, 8], [1, 1], [30, 100]], commands=["render-probe-facts", "render-probe", "G", "q"], settings=dict(hyperlinks=False, trueColor=True), theme=theme), indent=2))
            print("Retained private evidence:", evidence)
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
            os.close(master)


if __name__ == "__main__":
    run_pty()
