"""Native WHAM TLS fixture: host-resolved dummy OAuth, never real credits."""
import base64
import json
from pathlib import Path
import time

from .tls_proxy import TLSProxy


class CodexProxy(TLSProxy):
    def __init__(self, directory):
        self.claims = {"email": "native@example.org", "exp": int(time.time()) + 86400 * 30,
                       "https://api.openai.com/auth": {"chatgpt_account_id": "dummy-account-id", "chatgpt_plan_type": "pro"}}
        encoded = base64.urlsafe_b64encode(json.dumps(self.claims).encode()).decode().rstrip("=")
        self.token = "header." + encoded + ".dummy-signature"
        self.usage: dict[str, object] = dict(account=dict(email="wrong@example.org", plan="plus"),
                          rate_limit=dict(primary_window=dict(used_percent=23.5, limit_window_seconds=18000, reset_at=1700000000),
                                          secondary_window=dict(used_percent=110, limit_window_seconds=604800, reset_at=1700500000)))
        self.credits = dict(credits=[dict(id="native-credit", status="banked", granted_at="2026-01-01T12:30:00Z")])
        self.result: dict[str, object] = {}
        super().__init__(directory, "chatgpt.com")

    def seed_agent(self, directory):
        directory = Path(directory)
        directory.mkdir(exist_ok=True)
        (directory / "auth.json").write_text(json.dumps({"openai-codex": dict(
            type="oauth", access=self.token, refresh="dummy-refresh-never-used",
            expires=int(time.time() * 1000) + 86400 * 30 * 1000, accountId="dummy-account-id")}))
        (directory / "auth.json").chmod(0o600)
        model = dict(id="hermetic", name="Hermetic native", reasoning=False, input=["text"], contextWindow=8192, maxTokens=1024)
        replacement = dict(model, id="replacement", name="Replacement native")
        (directory / "models.json").write_text(json.dumps(dict(providers={"openai-codex": dict(
            baseUrl="https://chatgpt.com/backend-api/codex", api="openai-codex-responses", models=[model, replacement])})))
        return "openai-codex/hermetic"

    def response(self, method, target, headers, payload):
        assert headers.get("authorization") == "Bearer " + self.token, "host OAuth token was not resolved"
        assert headers.get("chatgpt-account-id") == "dummy-account-id", "account claim not applied"
        assert headers.get("originator") == "pig", "wrong PiG identity"
        row = dict(method=method, path=target, headers=headers)
        if payload:
            row["body"] = json.loads(payload)
        self.requests.append(row)
        if target == "/backend-api/wham/usage":
            assert method == "GET"
            data = self.usage
        elif target == "/backend-api/wham/rate-limit-reset-credits":
            assert method == "GET"
            data = self.credits
        else:
            assert method == "POST" and target == "/backend-api/wham/rate-limit-reset-credits/consume", (method, target)
            data = self.result
        return self.status, json.dumps(data).encode()
