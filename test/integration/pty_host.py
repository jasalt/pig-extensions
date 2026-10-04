"""Isolated interactive PiG in a real pseudo-terminal, for TUI-path tests.

Raw terminal bytes are evidence that the host emitted output to a terminal;
they do not prove that a graphical terminal displayed images.
"""
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import struct
import subprocess
import termios
import time

from .rpc import BIN


class PTYHost:
    def __init__(self, cwd, extensions, session, rows=30, cols=100, env=None, settings=None, args=None):
        self.cwd = Path(cwd)
        self.session = Path(session)
        agent = self.cwd / "agent"
        agent.mkdir(exist_ok=True)
        merged = dict(quietStartup=True)
        merged.update(settings or {})
        (agent / "settings.json").write_text(json.dumps(merged))
        base = dict(os.environ, PIG_HOME=str(self.cwd / "home"), PIG_CODING_AGENT_DIR=str(agent),
                    PI_CODING_AGENT_DIR=str(agent), PIG_USE_PI_DIRS="0", PIG_OFFLINE="1", PI_OFFLINE="1",
                    PI_SKIP_VERSION_CHECK="1", TERM="xterm-256color", COLORTERM="truecolor", HERDR_ENV="0")
        for key in ["TMUX", "TERM_PROGRAM", "KITTY_WINDOW_ID", "WEZTERM_PANE", "WEZTERM_EXECUTABLE",
                    "ITERM_SESSION_ID", "GHOSTTY_RESOURCES_DIR", "WT_SESSION", "PI_IMAGE_PROTOCOL"]:
            base.pop(key, None)
        for key in list(base):
            if key.endswith(("API_KEY", "ACCESS_TOKEN")) or "PUSHOVER" in key:
                del base[key]
        base.update(env or {})
        self.raw = bytearray()
        self.master, slave = pty.openpty()
        self.resize(rows, cols, fd=slave)

        def controlling_terminal():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)

        argv = [str(BIN), "--no-extensions", "--no-skills", "--no-prompt-templates",
                "--no-context-files", "--session", str(self.session), *(args or [])]
        for extension in extensions:
            argv += ["-e", str(extension)]
        self.proc = subprocess.Popen(argv, cwd=self.cwd, env=base, stdin=slave, stdout=slave,
                                     stderr=slave, preexec_fn=controlling_terminal)
        os.close(slave)
        self.pump_until(lambda: b"\x1b[?2004h" in self.raw)

    def resize(self, rows, cols, fd=None):
        fcntl.ioctl(self.master if fd is None else fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    def pump(self, timeout=0.05):
        readable, _, _ = select.select([self.master], [], [], timeout)
        if readable:
            try:
                self.raw.extend(os.read(self.master, 1 << 16))
            except OSError:
                pass

    def pump_until(self, predicate, timeout=60):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if predicate():
                return
            if self.proc.poll() is not None:
                raise AssertionError("PTY host exited: " + self.raw[-4000:].decode(errors="replace"))
            self.pump()
        raise AssertionError("PTY deadline: " + self.raw[-4000:].decode(errors="replace"))

    def settle(self, quiet=0.3, timeout=10):
        """Read until the terminal has been quiet for `quiet` seconds."""
        deadline = time.monotonic() + timeout
        last = len(self.raw)
        quiet_since = time.monotonic()
        while time.monotonic() < deadline:
            self.pump(0.05)
            if len(self.raw) != last:
                last = len(self.raw)
                quiet_since = time.monotonic()
            elif time.monotonic() - quiet_since >= quiet:
                return

    def write(self, data):
        os.write(self.master, data if isinstance(data, bytes) else data.encode())

    def wait_command(self, name, timeout=120):
        """Wait until slash autocomplete lists `name`, then clear the editor.

        The interactive host accepts input before extensions finish loading.
        """
        deadline = time.monotonic() + timeout
        while True:
            mark = len(self.raw)
            self.write("/" + name)
            try:
                self.pump_until(lambda: name.encode() in self.raw[mark:] and b"\x1b[?2026l" in self.raw[mark:], timeout=3)
                self.settle(0.3)
                listed = name.encode() in self.raw[mark:] and b"No matching" not in self.raw[mark:]
            except AssertionError:
                listed = False
            self.write("\x7f" * (len(name) + 1))
            self.settle(0.2)
            if listed and self._autocomplete_listed(name, mark):
                return
            if time.monotonic() > deadline:
                raise AssertionError(f"command /{name} never registered")
            time.sleep(0.5)

    def _autocomplete_listed(self, name, mark):
        # The typed text itself contains the name; require a second occurrence
        # (the suggestion row) in the frames after typing.
        return self.raw[mark:].count(name.encode()) >= 2

    def command(self, text):
        mark = len(self.raw)
        self.write(text + "\r")
        return mark

    def entries(self, kind=None):
        try:
            rows = [json.loads(line) for line in self.session.read_text().splitlines()]
        except (ValueError, FileNotFoundError):
            return []
        return [r for r in rows if kind is None or r.get("customType") == kind]

    def close(self):
        self.proc.terminate()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
        os.close(self.master)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
