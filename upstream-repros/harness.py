"""Stdlib-only helpers for the upstream reproductions.

Everything runs against PIG_BIN (default: `pig` on PATH) with a temporary
PIG_HOME/agent directory, no ambient extensions, and a local scripted
OpenAI-compatible model when one is needed. Nothing leaves 127.0.0.1.
"""
import fcntl
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import pty
import queue
import select
import shutil
import struct
import subprocess
import termios
import threading
import time

BIN = os.environ.get("PIG_BIN") or shutil.which("pig") or "pig"


def isolated_env(home, extra=None):
    home = Path(home)
    env = dict(os.environ, PIG_HOME=str(home / "home"), PIG_CODING_AGENT_DIR=str(home / "agent"),
               PIG_USE_PI_DIRS="0", PIG_OFFLINE="1", PI_OFFLINE="1", PI_SKIP_VERSION_CHECK="1")
    for key in list(env):
        if key.endswith(("API_KEY", "ACCESS_TOKEN")):
            del env[key]
    env.update(extra or {})
    (home / "agent").mkdir(parents=True, exist_ok=True)
    return env


class RPC:
    """Minimal `pig --mode rpc` driver."""

    def __init__(self, home, extensions, model=None, env=None, args=None):
        argv = [BIN, "--mode", "rpc", "--no-extensions", "--no-skills", "--no-prompt-templates",
                "--no-context-files", "--no-themes", "--no-session", *(args or [])]
        if model:
            argv += ["--model", model]
        for extension in extensions:
            argv += ["-e", str(extension)]
        self.events, self.lines, self.seq = [], queue.Queue(), 0
        self.proc = subprocess.Popen(argv, cwd=home, env=isolated_env(home, env), text=True,
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        threading.Thread(target=self._read, daemon=True).start()
        self.call("get_commands")

    def _read(self):
        for line in self.proc.stdout:
            try:
                self.lines.put(json.loads(line))
            except ValueError:
                pass
        self.lines.put({"eof": True})

    def send(self, kind, **fields):
        self.seq += 1
        self.proc.stdin.write(json.dumps(dict(type=kind, id=str(self.seq), **fields)) + "\n")
        self.proc.stdin.flush()
        return str(self.seq)

    def call(self, kind, timeout=120, **fields):
        identifier = self.send(kind, **fields)
        return self.until(lambda r: r.get("type") == "response" and r.get("id") == identifier, timeout)

    def until(self, predicate, timeout=60, since=None):
        """Return the first matching frame; with `since`, frames already
        received from that index count too."""
        if since is not None:
            for row in self.events[since:]:
                if predicate(row):
                    return row
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            try:
                row = self.lines.get(timeout=0.1)
            except queue.Empty:
                continue
            self.events.append(row)
            if row.get("eof"):
                raise RuntimeError("pig exited")
            if predicate(row):
                return row
        raise TimeoutError("RPC wait timed out")

    def notifications(self):
        return [r.get("message", "") for r in self.events
                if r.get("type") == "extension_ui_request" and r.get("method") == "notify"]

    def close(self):
        self.proc.stdin.close()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()


class PTY:
    """Interactive `pig` in a real pseudo-terminal."""

    def __init__(self, home, extensions, rows=30, cols=100, env=None, args=None):
        self.raw = bytearray()
        self.master, slave = pty.openpty()
        self.resize(rows, cols, slave)
        argv = [BIN, "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files",
                "--no-session", *(args or [])]
        for extension in extensions:
            argv += ["-e", str(extension)]
        base = dict(TERM="xterm-256color", COLORTERM="truecolor")
        base.update(env or {})

        def setup():
            os.setsid()
            fcntl.ioctl(0, termios.TIOCSCTTY, 0)

        self.proc = subprocess.Popen(argv, cwd=home, env=isolated_env(home, base), stdin=slave, stdout=slave,
                                     stderr=slave, preexec_fn=setup)
        os.close(slave)

    def resize(self, rows, cols, fd=None):
        fcntl.ioctl(self.master if fd is None else fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    def pump(self, seconds=0.05):
        if select.select([self.master], [], [], seconds)[0]:
            try:
                self.raw.extend(os.read(self.master, 1 << 16))
            except OSError:
                pass

    def until(self, predicate, timeout=60):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if predicate():
                return True
            self.pump()
        return False

    def settle(self, quiet=0.5, timeout=10):
        deadline, last, since = time.monotonic() + timeout, len(self.raw), time.monotonic()
        while time.monotonic() < deadline:
            self.pump()
            if len(self.raw) != last:
                last, since = len(self.raw), time.monotonic()
            elif time.monotonic() - since >= quiet:
                return

    def write(self, data):
        os.write(self.master, data.encode() if isinstance(data, str) else data)

    def wait_command(self, name, description, timeout=120):
        """Wait until slash autocomplete lists /name (by its description)."""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            mark = len(self.raw)
            self.write("/" + name)
            self.settle(0.3, 3)
            listed = description.encode() in self.raw[mark:]
            self.write("\x7f" * (len(name) + 1))
            self.settle(0.2, 3)
            if listed:
                return
            time.sleep(0.5)
        raise TimeoutError(f"/{name} never registered")

    def close(self):
        self.proc.terminate()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
        os.close(self.master)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()


class ScriptedModel:
    """Local OpenAI-compatible model; `script(messages)` returns reply text."""

    def __init__(self, home, script):
        self.requests = []
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                owner.requests.append(body)
                text = script(body["messages"])
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.end_headers()
                for delta, reason in [(dict(role="assistant", content=text), None), ({}, "stop")]:
                    chunk = dict(id="x", object="chat.completion.chunk", created=1, model="m",
                                 choices=[dict(index=0, delta=delta, finish_reason=reason)])
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                self.wfile.write(b"data: [DONE]\n\n")

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.model = "repro/model"
        agent = Path(home) / "agent"
        agent.mkdir(parents=True, exist_ok=True)
        (agent / "models.json").write_text(json.dumps(dict(providers=dict(repro=dict(
            baseUrl=f"http://127.0.0.1:{self.server.server_port}/v1", apiKey="dummy", api="openai-completions",
            models=[dict(id="model", name="Repro", reasoning=False, input=["text"], contextWindow=65536,
                         maxTokens=1024)])))))

    def close(self):
        self.server.shutdown()
        self.server.server_close()


def text_of(message):
    content = message.get("content")
    if isinstance(content, list):
        return "\n".join(part.get("text", "") for part in content if isinstance(part, dict))
    return content or ""


def result(issue, reproduced, detail):
    print(f"{issue}: {'REPRODUCED' if reproduced else 'NOT REPRODUCED'} — {detail}")
    return reproduced
