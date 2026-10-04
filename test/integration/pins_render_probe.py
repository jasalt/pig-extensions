"""Production Go-native renderer hypothesis probe; not pins acceptance."""
import json
from pathlib import Path
import tempfile

from .rpc import ROOT, RPC

FIXTURE = ROOT / "test/fixtures/pins-render-probe"

with tempfile.TemporaryDirectory(prefix="pins-render-facts-") as temp:
    cwd = Path(temp)
    agent = cwd / "agent"
    agent.mkdir()
    theme = json.loads((ROOT.parent / "PiG/tui/theme_dark.json").read_text())
    theme["name"] = "probe-custom"
    theme["colors"]["syntaxKeyword"] = "#123abc"
    theme_file = cwd / "probe-custom.json"
    theme_file.write_text(json.dumps(theme))
    (agent / "settings.json").write_text(json.dumps(dict(
        terminal=dict(hyperlinks=False, trueColor=True), theme="probe-custom")))
    with RPC(cwd, [FIXTURE], extra_args=["--theme", str(theme_file), "--use-theme", "probe-custom"],
             extra_env=dict(TERM="xterm-kitty", COLORTERM="truecolor")) as rpc:
        _, rows = rpc.call("prompt", message="/render-probe-facts")
        facts = [r["message"] for r in rows if r.get("method") == "notify"]
        assert len(facts) == 1, rows
        print(facts[0])
        _, rows = rpc.call("prompt", message="/render-probe")
        assert any(r.get("method") == "notify" and r.get("message") == "render-probe requires a terminal" for r in rows), rows
        assert not any(r.get("method") == "custom" for r in rows), "RPC opened terminal-only UI"
