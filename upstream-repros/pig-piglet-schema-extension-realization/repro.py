"""`pig piglet validate` accepts build.extensionRealization, but the schema
from `pig piglet schema` forbids it (buildSpec has additionalProperties:false
and no such property). PIG_BIN=... python3 repro.py"""
import json
from pathlib import Path
import subprocess
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import BIN, isolated_env, result  # noqa: E402

with tempfile.TemporaryDirectory() as home:
    env = isolated_env(home)
    schema = json.loads(subprocess.run([BIN, "piglet", "schema"], env=env, capture_output=True, text=True, check=True).stdout)
    build = schema["$defs"]["buildSpec"]
    in_schema = "extensionRealization" in build.get("properties", {})
    outcomes = {}
    for value in ["fused", "sometimes"]:
        path = Path(home) / f"{value}.yaml"
        path.write_text(f"name: probe\nbuild:\n  extensionRealization: {value}\n")
        run = subprocess.run([BIN, "piglet", "validate", str(path)], env=env, capture_output=True, text=True)
        outcomes[value] = (run.returncode, (run.stdout + run.stderr).strip().splitlines()[-1])
    result("pig-piglet-schema-extension-realization",
           not in_schema and build.get("additionalProperties") is False and outcomes["fused"][0] == 0 and outcomes["sometimes"][0] != 0,
           f"schema buildSpec properties={sorted(build.get('properties', {}))} additionalProperties={build.get('additionalProperties')}; "
           f"validate fused -> {outcomes['fused']}; validate sometimes -> {outcomes['sometimes']}")
