"""Local-only fixed-endpoint Pushover fixture; no live traffic."""
from urllib.parse import parse_qs

from .tls_proxy import TLSProxy


class PushoverProxy(TLSProxy):
    def __init__(self, directory):
        super().__init__(directory, "api.pushover.net")
        self.body = b'{"status":1}'

    def response(self, method, target, headers, payload):
        assert method == "POST" and target == "/1/messages.json", (method, target)
        self.requests.append(parse_qs(payload.decode(), keep_blank_values=True))
        return self.status, self.body
