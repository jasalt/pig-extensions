"""Actual native OAuth -> WHAM command path, intercepted entirely by local TLS."""
import json
from pathlib import Path
import re
import tempfile

from .codex_cases import status_card
from .codex_proxy import CodexProxy
from .rpc import ROOT, RPC
from .savelast import notification

EXT = ROOT / "extensions/codex-usage"

with tempfile.TemporaryDirectory(prefix="codex-native-proof-") as temp:
    cwd = Path(temp)
    with CodexProxy(cwd) as proxy:
        model = proxy.seed_agent(cwd / "agent")
        with RPC(cwd, [EXT], model=model, extra_env=dict(proxy.env, TZ="UTC", LC_ALL="en_US.UTF-8")) as rpc:
            for _ in range(2):
                _, rows = rpc.call("prompt", message="/codex-usage")
                card = status_card("native@example.org", "Pro")
                notification(rows, card, "info")
                assert [r["message"] for r in rows if r.get("method") == "notify" and r.get("notifyType") == "info"] == [card], rows
            _, rows = rpc.call("prompt", message="/codex-reset")
            notification(rows, "Banked Codex rate-limit resets:\nnative-credit  banked  granted 2026-01-01 12:30 UTC  expires never/unknown", "info")
            nonces = set()
            for result in [None, "reset", "already_redeemed", "nothing_to_reset", "no_credit"]:
                proxy.result = {} if result is None else dict(status=result, rate_limit_windows_reset=2)
                _, rows = rpc.call("prompt", message="/codex-reset native-credit")
                want = "Reset native-credit activated (0 rate-limit windows reset)." if result is None else (
                    "Reset native-credit activated (2 rate-limit windows reset)." if result == "reset" else
                    f"Reset native-credit result: {result} (2 rate-limit windows reset).")
                notification(rows, want, "info")
                post = [r for r in proxy.requests if r["method"] == "POST"][-1]
                assert post["path"] == "/backend-api/wham/rate-limit-reset-credits/consume", post
                body = post["body"]
                assert set(body) == {"credit_id", "redeem_request_id"} and body["credit_id"] == "native-credit", body
                nonce = body["redeem_request_id"]
                assert re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", nonce), nonce
                assert nonce not in nonces, "redemption ID reused"
                nonces.add(nonce)
            before = len(proxy.requests)
            proxy.result = dict(result="unexpected")
            _, rows = rpc.call("prompt", message="/codex-reset native-credit")
            notification(rows, "Activate Codex reset returned unexpected result: unexpected", "error")
            assert len(proxy.requests) == before + 1, "unexpected result retried or refreshed"
            assert not any(r.get("method") == "notify" and r.get("notifyType") == "info" for r in rows), rows
            for secret in [proxy.token, "dummy-account-id", "dummy-refresh-never-used"]:
                assert secret not in json.dumps(rpc.events), "native credential leaked into RPC output"
        assert rpc.proc.returncode == 0, rpc.errors
        assert {r["path"] for r in proxy.requests} == {
            "/backend-api/wham/usage", "/backend-api/wham/rate-limit-reset-credits",
            "/backend-api/wham/rate-limit-reset-credits/consume"}, proxy.requests
print("PASS native Codex: actual PiG OAuth, WHAM paths/headers/claims, all acknowledgements, unique v4 redemption IDs, no credential output")
