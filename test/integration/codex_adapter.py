"""Local Codex adapter exercising the unchanged production PiG factory."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading


class CodexAdapter:
    def __init__(self):
        self.requests = []
        self.request_cond = threading.Condition()
        self.usage_status = 200
        self.usage: dict[str, object] = dict(account=dict(email="fixture@example.org", plan="plus"),
                          rate_limit=dict(primary_window=dict(used_percent=23.5, limit_window_seconds=18000, reset_at=1700000000),
                                          secondary_window=dict(used_percent=110, limit_window_seconds=604800, reset_at=1700500000)))
        self.credits = dict(credits=[dict(id="later", status="banked", granted_at="2026-01-01T12:30:00+02:00"),
                                    dict(id="early", status="banked", granted_at="2026-01-01T12:30:00Z", expires_at="2026-02-01T00:00:00Z")])
        self.reset_result = dict(result="reset", rate_limit_windows_reset=2)
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                pass

            def do_GET(self):
                owner.record(dict(method="GET", path=self.path, headers=dict(self.headers)))
                if self.path == "/v1/codex/usage":
                    payload, code = owner.usage, owner.usage_status
                elif self.path == "/v1/codex/resets":
                    payload, code = owner.credits, 200
                else:
                    payload, code = dict(error="unknown fixture endpoint"), 404
                self.send_response(code)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(payload).encode())

            def do_POST(self):
                payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                owner.record(dict(method="POST", path=self.path, headers=dict(self.headers), body=payload))
                self.send_response(200)
                if self.path == "/v1/chat/completions":
                    self.send_header("Content-Type", "text/event-stream")
                    self.end_headers()
                    for delta, reason in [(dict(content="Fixture response."), None), ({}, "stop")]:
                        row = dict(id="fixture", object="chat.completion.chunk", created=1, model="hermetic",
                                   choices=[dict(index=0, delta=delta, finish_reason=reason)])
                        self.wfile.write(("data: " + json.dumps(row) + "\n\n").encode())
                    self.wfile.write(b"data: [DONE]\n\n")
                else:
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    self.wfile.write(json.dumps(owner.reset_result).encode())

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.model = "fixture/hermetic"
        self.config = dict(providers=dict(fixture=dict(
            baseUrl=f"http://127.0.0.1:{self.server.server_port}/v1", apiKey="dummy-codex-key",
            headers={"X-Fixture-Auth": "dummy-configured-header"}, api="openai-completions",
            models=[dict(id="hermetic", name="Hermetic", reasoning=False, input=["text"], contextWindow=8192, maxTokens=1024),
                    dict(id="replacement", name="Replacement", reasoning=False, input=["text"], contextWindow=8192, maxTokens=1024)])))

    def record(self, request):
        with self.request_cond:
            self.requests.append(request)
            self.request_cond.notify_all()

    def wait_request(self, path, since, timeout=10):
        with self.request_cond:
            return self.request_cond.wait_for(
                lambda: any(r["path"] == path for r in self.requests[since:]), timeout)

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
