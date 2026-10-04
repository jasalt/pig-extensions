"""Host exec timeout does not stop the process tree. PIG_BIN=... python3 repro.py"""
import os
from pathlib import Path
import re
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import RPC, result  # noqa: E402

HERE = Path(__file__).resolve().parent


def alive(pid):
    try:
        return Path(f"/proc/{pid}/stat").read_text().split()[2] != "Z"
    except OSError:
        return False


with tempfile.TemporaryDirectory() as home:
    pidfile = Path(home) / "child.pid"
    with RPC(home, [HERE]) as rpc:
        rpc.call("prompt", message=f"/exec-probe {pidfile}")
        note = next(n for n in rpc.notifications() if n.startswith(("elapsed=", "exec error")))
        pid = int(pidfile.read_text())
        elapsed = float(re.search(r"elapsed=([\d.]+)s", note).group(1))
        result("pig-exec-timeout-process-tree", elapsed > 5,
               f"timeout 1s; call returned after {elapsed:.1f}s ({note}); "
               f"background child {'still alive' if alive(pid) else 'gone'} after return")
