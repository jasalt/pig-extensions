"""RPC mode: an extension's ctx.Compact() silently does nothing, while the
client's own `compact` command works. PIG_BIN=... python3 repro.py"""
import json
from pathlib import Path
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import RPC, ScriptedModel, result  # noqa: E402

HERE = Path(__file__).resolve().parent

with tempfile.TemporaryDirectory() as home:
    model = ScriptedModel(home, lambda messages: "reply " + "x" * 400)
    (Path(home) / "agent/settings.json").write_text(json.dumps(dict(compaction=dict(keepRecentTokens=1))))
    try:
        with RPC(home, [HERE], model=model.model) as rpc:
            for turn in range(3):
                mark = len(rpc.events)
                rpc.call("prompt", message=f"turn {turn} " + "y" * 400)
                rpc.until(lambda r: r.get("type") == "agent_settled", since=mark)
            rpc.call("prompt", message="/compact-now")
            extension_view = [n for n in rpc.notifications() if "compaction" in n or "callbacks" in n]
            client = rpc.call("compact")
            result("pig-rpc-extension-compact-noop",
                   "no compaction within 5s" in extension_view and client.get("success") is True,
                   f"extension: {extension_view}; client compact command success={client.get('success')}")
    finally:
        model.close()
