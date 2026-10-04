"""Run every PiG reproduction against PIG_BIN and print one line per issue.

PIG_BIN=/path/to/pig PIG_GIT_CHECKOUT=/path/to/PiG-git-checkout python3 run_all.py

PIG_GIT_CHECKOUT is only needed for pig-source-root-not-git (skipped without it).
"""
import os
from pathlib import Path
import subprocess
import sys

HERE = Path(__file__).resolve().parent
REPROS = [
    "pig-exec-timeout-process-tree",
    "pig-go-sdk-terminal-capabilities",
    "pig-go-sdk-overlay-height-resize",
    "pig-piglet-schema-extension-realization",
    "pig-source-root-not-git",
    "pig-rpc-extension-compact-noop",
    "pig-send-during-compaction",  # expected NOT REPRODUCED: checked, not an issue
    "pig-followup-event-order",  # observation for extension authors, not a bug
]

failed = False
for name in REPROS:
    if name == "pig-source-root-not-git" and not os.environ.get("PIG_GIT_CHECKOUT"):
        print(f"{name}: SKIPPED — set PIG_GIT_CHECKOUT")
        continue
    run = subprocess.run([sys.executable, "repro.py"], cwd=HERE / name, capture_output=True, text=True, timeout=900)
    lines = [l for l in run.stdout.splitlines() if l.startswith(name)]
    if run.returncode or not lines:
        failed = True
        print(f"{name}: ERROR — {(run.stdout + run.stderr).strip().splitlines()[-1:]}")
    else:
        print(lines[-1])
sys.exit(1 if failed else 0)
