"""Actual host cancellation while notify_human has an in-flight HTTPS POST."""
import json
from pathlib import Path
import tempfile

from .pushover_proxy import PushoverProxy
from .rpc import ROOT, RPC
from .tool_provider import ToolProvider

for action in ["abort", "disconnect"]:
    with tempfile.TemporaryDirectory(prefix="notify-cancel-proof-") as temp:
        cwd = Path(temp)
        agent = cwd / "agent"
        agent.mkdir()
        (agent / "notify-pushover.json").write_text(json.dumps(
            dict(userKey="dummy-cancel-user", appToken="dummy-cancel-token")))
        with ToolProvider("notify_human", dict(message="cancel fixture")) as provider, PushoverProxy(cwd) as proxy:
            proxy.hold = True
            (agent / "models.json").write_text(json.dumps(provider.config))
            rpc = RPC(cwd, [ROOT / "extensions/notify-pushover"],
                      extra_env=dict(proxy.env, NO_PROXY="127.0.0.1,localhost"), model=provider.model)
            try:
                rpc.call("prompt", message="Run the pending fixture tool.")
                assert proxy.started.wait(10), "tool POST did not start"
                if action == "abort":
                    since = len(rpc.events)
                    rpc.call("abort")
                    rpc.wait_event("agent_end", since=since)
                else:
                    rpc.close()
                assert proxy.disconnected.wait(5), "HTTP connection survived host cancellation"
                assert len(proxy.requests) == 1, "retry or late POST occurred"
            finally:
                if rpc.proc.poll() is None:
                    rpc.close()
            assert rpc.proc.returncode == 0, rpc.errors
    print("PASS notify_human actual host " + action + ": HTTP teardown, no retries/orphan work")
