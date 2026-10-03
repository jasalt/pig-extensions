"""Require a compiling whole-log mutant to fail the real active-branch test."""
from pathlib import Path
import os
import shutil
import subprocess
import tempfile

from test.integration.rpc import ROOT

with tempfile.TemporaryDirectory(prefix="savelast-mutant-") as temp:
    module = Path(temp) / "savelast"
    shutil.copytree(ROOT / "extensions/savelast", module)
    source = module / "extension.go"
    original = source.read_text()
    needle = "ctx.SessionManager().GetBranch(nil)"
    assert original.count(needle) == 1
    source.write_text(original.replace(needle, "ctx.SessionManager().GetEntries()"))
    subprocess.run(["go", "test", "./..."], cwd=module, check=True)
    result = subprocess.run(["python3", "-m", "test.integration.savelast"], cwd=ROOT,
                            env=dict(os.environ, SAVELAST_ROOT=str(module)),
                            capture_output=True, text=True)
    assert result.returncode != 0, "whole-log mutant survived real RPC acceptance"
    assert "ABANDONED" in result.stderr, result.stderr
    print("PASS mutation: compiling GetEntries mutant fails exact active-branch bytes")
