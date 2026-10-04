"""A non-git PIG_SOURCE_ROOT (e.g. a release tarball or `git archive`
export) fails with "resolve Pig source revision: exit status 128".

PIG_BIN=... PIG_GIT_CHECKOUT=/path/to/PiG python3 repro.py
"""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import BIN, isolated_env, result  # noqa: E402

checkout = os.environ["PIG_GIT_CHECKOUT"]
extension = Path(__file__).resolve().parent.parent / "pig-go-sdk-overlay-height-resize"

with tempfile.TemporaryDirectory() as home:
    export = Path(home) / "pig-src"
    export.mkdir()
    # Same commit as the binary, as plain files.
    archive = subprocess.run(["git", "-C", checkout, "archive", "HEAD"], capture_output=True, check=True).stdout
    subprocess.run(["tar", "-x", "-C", str(export)], input=archive, check=True)
    shutil.copytree(extension, Path(home) / "ext")
    piglet = Path(home) / "probe.yaml"
    piglet.write_text("name: probe\nextensions:\n  - name: ext\n    origins: [local:./ext]\n"
                      "build:\n  extensionRealization: fused\n")
    run = subprocess.run([BIN, "piglet", "build", str(piglet), "--format", "binary", "--out", str(Path(home) / "out")],
                         cwd=home, env=isolated_env(home, dict(PIG_SOURCE_ROOT=str(export))),
                         capture_output=True, text=True, timeout=600)
    message = next((l for l in (run.stdout + run.stderr).splitlines() if "revision" in l), "")
    result("pig-source-root-not-git", run.returncode != 0 and "exit status 128" in message,
           f"exit {run.returncode}: {message.strip()}")
