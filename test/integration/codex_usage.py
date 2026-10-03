"""Real PiG Codex commands, model auth, reset bodies and lifecycle refreshes."""
import json
from pathlib import Path
import tempfile

from .codex_adapter import CodexAdapter
from .rpc import ROOT, RPC
from .savelast import notification

EXT = ROOT / "extensions/codex-usage"
CARD = "\n".join([
    "ChatGPT Codex status",
    "Visit https://chatgpt.com/codex/settings/usage for up-to-date information on rate limits and credits.",
    "Account:       fixture@example.org (Plus)",
    "5h limit:      [███████████████░░░░░] 76.5% left (resets 22:13)",
    "Weekly limit: [░░░░░░░░░░░░░░░░░░░░] 0% left (resets 17:06 on Nov 20)",
])

with tempfile.TemporaryDirectory(prefix="codex-adapter-proof-") as temp, CodexAdapter() as adapter:
    cwd = Path(temp)
    agent = cwd / "agent"
    agent.mkdir()
    (agent / "models.json").write_text(json.dumps(adapter.config))
    with RPC(cwd, [EXT], model=adapter.model,
             extra_env=dict(TZ="UTC", LC_ALL="en_US.UTF-8", NO_PROXY="127.0.0.1,localhost")) as rpc:
        for _ in range(2):
            _, rows = rpc.call("prompt", message="/codex-usage")
            notification(rows, CARD, "info")
            cards = [r["message"] for r in rows if r.get("method") == "notify" and r.get("notifyType") == "info"]
            assert cards == [CARD], rows
        _, rows = rpc.call("prompt", message="/codex-reset")
        notice = [r["message"] for r in rows if r.get("method") == "notify" and r.get("notifyType") == "info"]
        assert len(notice) == 1 and notice[0].startswith("Banked Codex rate-limit resets:\nearly"), rows
        assert "later  banked  granted 2026-01-01 10:30 UTC  expires never/unknown" in notice[0]
        _, rows = rpc.call("prompt", message="/codex-reset exact-id")
        notification(rows, "Reset exact-id activated (2 rate-limit windows reset).", "info")
        posts = [r for r in adapter.requests if r["method"] == "POST"]
        assert len(posts) == 1 and posts[0]["path"] == "/v1/codex/reset", posts
        assert posts[0]["body"] == dict(credit_id="exact-id"), posts
        before = len(adapter.requests)
        _, rows = rpc.call("prompt", message="/codex-reset not an id")
        notification(rows, "Usage: /codex-reset [reset-id]", "error")
        assert len(adapter.requests) == before, "invalid ID issued HTTP"
        for request in adapter.requests:
            assert request["headers"]["Authorization"] == "Bearer dummy-codex-key"
            assert request["headers"]["X-Fixture-Auth"] == "dummy-configured-header"
        before = len(adapter.requests)
        rpc.call("new_session")
        assert len(adapter.requests) > before, "session start did not refresh"
        before = len(adapter.requests)
        rpc.call("set_model", provider="fixture", modelId="replacement")
        assert len(adapter.requests) > before, "model selection did not refresh"
        # A hermetic real turn must trigger the settlement refresh too.
        marker = len(rpc.events)
        before = len(adapter.requests)
        rpc.call("prompt", message="Fixture turn.")
        rpc.wait_event("agent_end", since=marker)
        assert adapter.wait_request("/v1/codex/usage", before), "settlement did not refresh"
        since = adapter.requests[before:]
        assert any(r["path"] == "/v1/chat/completions" for r in since), since
        assert any(r["path"] == "/v1/codex/usage" for r in since), since
        adapter.usage_status = 403
        adapter.usage = dict(error="denied dummy-codex-key dummy-configured-header")
        _, rows = rpc.call("prompt", message="/codex-usage")
        notification(rows, "Codex request failed (HTTP 403)", "error")
        assert not any(r.get("method") == "notify" and r.get("notifyType") == "info" for r in rows), rows
        for secret in ["dummy-codex-key", "dummy-configured-header"]:
            assert secret not in json.dumps(rows), "credential leaked"
        _, rows = rpc.call("get_messages")
        assert not any("customType" in m for m in rows[-1].get("data", {}).get("messages", [])), "notification persisted"
    assert rpc.proc.returncode == 0, rpc.errors

with tempfile.TemporaryDirectory(prefix="codex-no-model-proof-") as temp:
    with RPC(Path(temp), [EXT]) as rpc:
        _, rows = rpc.call("prompt", message="/codex-usage")
        # This checkout supplies an unknown/unknown placeholder model in a
        # fresh RPC home. It is present but unauthenticated, not a nil model.
        notification(rows, "No authentication available for unknown", "error")
        _, rows = rpc.call("prompt", message="/codex-reset not an id")
        notification(rows, "Usage: /codex-reset [reset-id]", "error")
print("PASS Codex real PiG adapter auth/headers, exact cards/reset POST, lifecycle refresh, errors, no persisted notices")
