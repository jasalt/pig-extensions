"""Hermetic OpenAI-compatible streaming provider for actual host tool dispatch."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading


class ToolProvider:
    def __init__(self, name, arguments):
        self.requests = []
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                owner.requests.append(body)
                messages = body["messages"]
                has_result = any(m.get("role") == "tool" for m in messages)
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Connection", "close")
                self.end_headers()
                if has_result:
                    delta = dict(content="Tool result received.")
                    reason = "stop"
                else:
                    delta = dict(role="assistant", tool_calls=[dict(index=0, id="fixture-call",
                                 type="function", function=dict(name=name, arguments=json.dumps(arguments)))])
                    reason = "tool_calls"
                for chunk in [dict(delta=delta, finish_reason=None),
                              dict(delta={}, finish_reason=reason)]:
                    data = dict(id="fixture-stream", object="chat.completion.chunk", created=1,
                                model="hermetic", choices=[dict(index=0, **chunk)])
                    self.wfile.write(("data: " + json.dumps(data) + "\n\n").encode())
                self.wfile.write(b"data: [DONE]\n\n")
                self.wfile.flush()

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.model = "fixture/hermetic"
        self.config = dict(providers=dict(fixture=dict(
            baseUrl=f"http://127.0.0.1:{self.server.server_port}/v1",
            apiKey="dummy-provider-key", api="openai-completions", models=[dict(
                id="hermetic", name="Hermetic", reasoning=False, input=["text"],
                contextWindow=8192, maxTokens=1024)])))

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
