"""Production notifier command path via local TLS proxy, never real delivery."""
import json
from pathlib import Path
import tempfile

from .pushover_proxy import PushoverProxy
from .rpc import ROOT, RPC
from .savelast import notification

EXT = ROOT / "extensions/notify-pushover"

with tempfile.TemporaryDirectory(prefix="notify-proof-") as temp:
    cwd = Path(temp)
    agent = cwd / "agent"
    agent.mkdir()
    with PushoverProxy(cwd) as proxy:
        with RPC(cwd, [EXT, ROOT / "extensions/savelast"], extra_env=proxy.env) as rpc:
            _, rows = rpc.call("prompt", message="/notify-human-test no creds")
            notification(rows, "Pushover credentials are not configured.", "error")
            assert not proxy.requests
            (agent / "notify-pushover.json").write_text(json.dumps(
                dict(userKey="dummy-user +&", appToken="dummy-token/+", device="fixture")))
            _, rows = rpc.call("prompt", message="/notify-human-test late config 決定")
            notification(rows, "Pushover test sent (200).", "info")
            assert len(proxy.requests) == 1
            assert proxy.requests[0] == dict(user=["dummy-user +&"], token=["dummy-token/+"],
                                            device=["fixture"], title=["PiG human notification test"],
                                            message=["late config 決定"], priority=["0"])
            proxy.status = 302
            proxy.body = b"dummy-user +& dummy-token/+ dummy-token%2F%2B"
            _, rows = rpc.call("prompt", message="/notify-human-test redirect")
            notification(rows, "Pushover test failed: Pushover failed: 302", "error")
            assert len(proxy.requests) == 2, "redirect/retry occurred"
            messages = "\n".join(r.get("message", "") for r in rows)
            for secret in ["dummy-user +&", "dummy-token/+", "dummy-token%2F%2B"]:
                assert secret not in messages, "secret leaked"
            # Session start must refresh cached credentials, not import old state.
            (agent / "notify-pushover.json").write_text(json.dumps(
                dict(userKey="replacement-user", appToken="replacement-token")))
            rpc.call("new_session")
            proxy.status, proxy.body = 200, b'{"status":1}'
            _, rows = rpc.call("prompt", message="/notify-human-test")
            notification(rows, "Pushover test sent (200).", "info")
            assert len(proxy.requests) == 3
            assert proxy.requests[-1]["user"] == ["replacement-user"]
            assert proxy.requests[-1]["token"] == ["replacement-token"]
            assert "device" not in proxy.requests[-1]
            _, rows = rpc.call("prompt", message="/savelast")
            notification(rows, "No agent message found to save", "warning")
    print("PASS notify-pushover real RPC: missing/late config, exact POST, redirect/redaction, new session, coexistence")
