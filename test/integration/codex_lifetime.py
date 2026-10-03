"""Actual caller replacement/disconnect while a native usage response is held."""
import json
from pathlib import Path
import tempfile

from .codex_cases import status_card
from .codex_proxy import CodexProxy
from .rpc import ROOT, RPC
from .savelast import notification

for action in ["model", "session", "disconnect"]:
    with tempfile.TemporaryDirectory(prefix="codex-lifetime-proof-") as temp:
        cwd = Path(temp)
        with CodexProxy(cwd) as proxy:
            model = proxy.seed_agent(cwd / "agent")
            rpc = RPC(cwd, [ROOT / "extensions/codex-usage"], model=model,
                      extra_env=dict(proxy.env, TZ="UTC", LC_ALL="en_US.UTF-8"))
            try:
                _, rows = rpc.call("prompt", message="/codex-usage")
                notification(rows, status_card("native@example.org", "Pro"), "info")
                proxy.started.clear()
                proxy.disconnected.clear()
                good = proxy.usage
                proxy.usage = dict(rate_limit=dict(primary_window=dict(
                    used_percent=99, limit_window_seconds=18000, reset_at=1700000000)))
                proxy.hold = True
                marker = len(rpc.events)
                pending = rpc.send("prompt", message="/codex-usage")
                assert proxy.started.wait(10), "held usage GET did not start"
                # The server already captured hold and the old body. New epoch
                # requests must receive fresh data while the old socket is held.
                proxy.hold = False
                proxy.usage = good
                if action == "model":
                    rpc.call("set_model", provider="openai-codex", modelId="replacement")
                elif action == "session":
                    rpc.call("new_session")
                else:
                    rpc.close()
                assert proxy.disconnected.wait(5), "obsolete HTTP connection survived lifecycle change"
                if action != "disconnect":
                    rpc.wait_response(pending)
                    obsolete = [r for r in rpc.events[marker:] if r.get("method") == "notify"]
                    assert not obsolete, "obsolete success/error was displayed: " + json.dumps(obsolete)
                    assert "1%/5h" not in json.dumps(rpc.events[marker:]), "obsolete footer status published"
                    _, rows = rpc.call("prompt", message="/codex-usage")
                    notification(rows, status_card("native@example.org", "Pro"), "info")
                for secret in [proxy.token, "dummy-account-id", "dummy-refresh-never-used"]:
                    assert secret not in json.dumps(rpc.events), "credential leaked"
            finally:
                if rpc.proc.poll() is None:
                    rpc.close()
            assert rpc.proc.returncode == 0, rpc.errors
            assert "1%/5h" not in json.dumps(rpc.events[marker:]), "obsolete footer status at shutdown"
            for secret in [proxy.token, "dummy-account-id", "dummy-refresh-never-used"]:
                assert secret not in json.dumps(rpc.events), "credential leaked in final frames"
    print("PASS Codex actual host " + action + ": old HTTP cancelled, no stale notification, fresh epoch usable")
