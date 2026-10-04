"""BTW feasibility probe: a Go extension runs an isolated child coding session.

Run: python3 -m test.integration.btw_child_probe. Not a BTW port.
"""
import json
from pathlib import Path
import tempfile

from .rpc import ROOT, RPC
from .scripted_provider import ScriptedProvider, text_of

FIXTURE = ROOT / "test/fixtures/btw-child-session-probe"


def run():
    with tempfile.TemporaryDirectory(prefix="btw-probe-") as temp:
        cwd = Path(temp)
        with ScriptedProvider(lambda messages: ("text", "SIDE-ANSWER to: " + text_of(messages[-1]))) as provider:
            (cwd / "agent").mkdir()
            (cwd / "agent/models.json").write_text(json.dumps(provider.config))
            with RPC(cwd, [FIXTURE], model=provider.model) as rpc:
                rpc.call("prompt", message="Parent question about X")
                rpc.wait_event("agent_end")
                _, rows = rpc.call("prompt", message="/btw-probe what is 2+2?")
                found = [(r.get("notifyType"), r.get("message")) for r in rows if r.get("method") == "notify"]
                assert ("info", "btw-probe reply: SIDE-ANSWER to: what is 2+2?") in found, found
            child = provider.snapshot()[-1]
            assert [(m["role"], text_of(m)) for m in child["messages"]] == [
                ("system", "You answer side questions briefly."), ("user", "what is 2+2?")], child["messages"]
            assert sorted(t["function"]["name"] for t in child.get("tools", [])) == ["find", "grep", "ls", "read"]
    print("PASS BTW feasibility: Go extension child session with parent model/credentials, isolated context, read-only built-ins, no child extensions")


if __name__ == "__main__":
    run()
