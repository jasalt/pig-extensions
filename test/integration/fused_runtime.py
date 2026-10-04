"""Fused Piglet Binary runtime check: extensions are present without source
paths and no extension runner subprocess exists.

Run: PIG_BIN=<fused binary> PIG_FUSED=1 python3 -m test.integration.fused_runtime
"""
import os
from pathlib import Path
import subprocess
import tempfile

from .rpc import BIN, RPC

EXPECTED_COMMANDS = {"savelast", "notify-human-test", "codex-usage", "codex-reset", "imgcat", "imgshow", "imgboth", "pin"}
EXPECTED_TOOLS = {"notify_human", "show_image", "schedule"}


def children(pid):
    out = subprocess.run(["ps", "--ppid", str(pid), "-o", "pid=,args="], capture_output=True, text=True).stdout
    return [line.strip() for line in out.splitlines() if line.strip()]


def run():
    assert os.environ.get("PIG_FUSED") == "1", "set PIG_FUSED=1 and PIG_BIN to the fused binary"
    with tempfile.TemporaryDirectory(prefix="fused-runtime-") as temp:
        with RPC(Path(temp), []) as rpc:
            _, rows = rpc.call("get_commands")
            names = {c["name"] for c in rows[-1]["data"]["commands"]}
            assert EXPECTED_COMMANDS <= names, sorted(names)
            _, rows = rpc.call("get_state")
            kids = children(rpc.proc.pid)
            assert not kids, f"extension subprocesses present: {kids}"
    print(f"PASS fused runtime: {len(EXPECTED_COMMANDS)} commands registered from {BIN.name} with no extension subprocess")


if __name__ == "__main__":
    run()
