"""Synchronized, isolated real-PiG RPC driver. No provider requests required."""
import json
import os
from pathlib import Path
import queue
import subprocess
import threading

ROOT = Path(__file__).resolve().parents[2]
BIN = Path(os.environ.get("PIG_BIN", ROOT.parent / "PiG/bin/pig")).resolve()


class RPC:
    def __init__(self, directory, extensions, session=None, extra_env=None, model=None):
        directory = Path(directory)
        self.events = []
        self.lines = queue.Queue()
        self.errors = []
        env = dict(os.environ, PIG_HOME=str(directory / "home"),
                   PIG_CODING_AGENT_DIR=str(directory / "agent"),
                   PI_OFFLINE="1", PI_SKIP_VERSION_CHECK="1")
        # Do not inherit provider credentials into functional tests.
        for key in list(env):
            if key.endswith(("API_KEY", "ACCESS_TOKEN")) or "PUSHOVER" in key:
                del env[key]
        if extra_env:
            env.update(extra_env)
        args = [str(BIN), "--mode", "rpc", "--no-extensions", "--no-skills",
                "--no-prompt-templates", "--no-context-files", "--no-themes"]
        if model:
            args += ["--model", model]
        if session:
            args += ["--session", str(session)]
        else:
            args += ["--no-session"]
        for extension in extensions:
            args += ["-e", str(extension)]
        self.proc = subprocess.Popen(args, cwd=directory, env=env, text=True,
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=subprocess.PIPE)
        self.threads = [threading.Thread(target=self._read, daemon=True),
                        threading.Thread(target=self._errors, daemon=True)]
        for thread in self.threads:
            thread.start()
        self.seq = 0
        self.call("get_commands")

    def _read(self):
        assert self.proc.stdout is not None
        for line in self.proc.stdout:
            try:
                self.lines.put(json.loads(line))
            except ValueError:
                self.lines.put({"unparsed": line})
        self.lines.put({"eof": True})

    def _errors(self):
        assert self.proc.stderr is not None
        self.errors.extend(self.proc.stderr)

    def call(self, kind, **fields):
        self.seq += 1
        identifier = str(self.seq)
        assert self.proc.stdin is not None
        self.proc.stdin.write(json.dumps(dict(type=kind, id=identifier, **fields)) + "\n")
        self.proc.stdin.flush()
        seen = []
        while True:
            try:
                row = self.lines.get(timeout=90)
            except queue.Empty:
                raise AssertionError(f"RPC timeout: {seen!r}; stderr={self.errors!r}")
            seen.append(row)
            self.events.append(row)
            if row.get("eof"):
                raise AssertionError(f"RPC exited: {seen!r}; stderr={self.errors!r}")
            if row.get("type") == "response" and row.get("id") == identifier:
                assert row.get("success"), row
                return row, seen

    def wait_event(self, kind, since=None):
        seen = [] if since is None else self.events[since:]
        for row in seen:
            if row.get("type") == kind:
                return row, seen
        while True:
            try:
                row = self.lines.get(timeout=30)
            except queue.Empty:
                raise AssertionError(f"event timeout: {kind}; {seen!r}; stderr={self.errors!r}")
            seen.append(row)
            self.events.append(row)
            if row.get("eof"):
                raise AssertionError(f"RPC exited: {seen!r}; stderr={self.errors!r}")
            if row.get("type") == kind:
                return row, seen

    def close(self):
        assert self.proc.stdin is not None
        assert self.proc.stdout is not None
        assert self.proc.stderr is not None
        self.proc.stdin.close()
        try:
            self.proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            self.proc.kill()
            self.proc.wait()
        for thread in self.threads:
            thread.join(timeout=2)
        self.proc.stdout.close()
        self.proc.stderr.close()

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()


def seed(path, cwd, entries):
    header = dict(type="session", version=3, id="offline-proof", cwd=str(cwd),
                  timestamp="2026-01-01T00:00:00.000Z")
    Path(path).write_text("".join(json.dumps(row) + "\n" for row in [header, *entries]))


def assistant(identifier, parent, text, reason="stop"):
    usage = dict(input=0, output=0, cacheRead=0, cacheWrite=0, totalTokens=0,
                 cost=dict(input=0, output=0, cacheRead=0, cacheWrite=0, total=0))
    return dict(type="message", id=identifier, parentId=parent,
                timestamp="2026-01-01T00:00:00.000Z", message=dict(
                    role="assistant", api="openai-responses", provider="openai",
                    model="gpt-4.1", content=text, stopReason=reason,
                    timestamp=1, usage=usage))
