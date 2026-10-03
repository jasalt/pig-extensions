"""Actual PiG model -> notify_human -> result round trip, with local-only HTTP."""
import json
from pathlib import Path
import tempfile

from .pushover_proxy import PushoverProxy
from .rpc import ROOT, RPC
from .tool_provider import ToolProvider

arguments = dict(message="Human decision 決定", title="Tool fixture", priority=-1,
                 url="https://example.invalid/question?a=+", urlTitle="Question")
with tempfile.TemporaryDirectory(prefix="notify-tool-proof-") as temp:
    cwd = Path(temp)
    agent = cwd / "agent"
    agent.mkdir()
    (agent / "notify-pushover.json").write_text(json.dumps(
        dict(userKey="dummy-tool-user", appToken="dummy-tool-token")))
    with ToolProvider("notify_human", arguments) as provider, PushoverProxy(cwd) as proxy:
        (agent / "models.json").write_text(json.dumps(provider.config))
        with RPC(cwd, [ROOT / "extensions/notify-pushover"],
                 extra_env=dict(proxy.env, NO_PROXY="127.0.0.1,localhost"), model=provider.model) as rpc:
            rpc.call("prompt", message="Run the fixture tool.")
            _, rows = rpc.wait_event("agent_end")
            ends = [r for r in rows if r.get("type") == "tool_execution_end"]
            assert len(ends) == 1, rows
            result = ends[0]
            assert result.get("toolName") == "notify_human" and not result.get("isError"), result
            assert len(proxy.requests) == 1, proxy.requests
            request = proxy.requests[0]
            assert request["message"] == [arguments["message"]]
            assert request["priority"] == ["-1"]
            assert request["url"] == [arguments["url"]]
            assert request["url_title"] == [arguments["urlTitle"]]
            tool_result = result["result"]
            assert tool_result["details"] == dict(status=200, device=None), tool_result
            assert tool_result["content"][0]["type"] == "text", tool_result
            assert "Pushover notification sent (200)" in tool_result["content"][0]["text"]
            assert len(provider.requests) == 2, provider.requests
            delivered = [m for m in provider.requests[-1]["messages"] if m.get("role") == "tool"]
            assert len(delivered) == 1, delivered
            assert "Pushover notification sent (200)" in delivered[0]["content"]
            serialized = json.dumps(rows) + json.dumps(delivered)
            for secret in ["dummy-tool-user", "dummy-tool-token"]:
                assert secret not in serialized, "credential leaked into tool results"
    print("PASS notify_human real host tool dispatch, exact form, result/details/provider delivery")
