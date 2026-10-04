"""Hermetic OpenAI-compatible provider whose reply depends on the last message.

`script(messages)` returns ("tool", name, arguments) or ("text", content).
Requests are recorded. No network beyond 127.0.0.1.
"""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading


def text_of(message):
    content = message.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "\n".join(part.get("text", "") for part in content if isinstance(part, dict))
    return ""


class ScriptedProvider:
    def __init__(self, script):
        self.requests = []
        self.lock = threading.Lock()
        owner = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                pass

            def do_POST(self):
                body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                with owner.lock:
                    owner.requests.append(body)
                reply = script(body["messages"])
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Connection", "close")
                self.end_headers()
                if reply[0] == "tool":
                    _, name, arguments = reply
                    delta = dict(role="assistant", tool_calls=[dict(index=0, id=f"call-{len(owner.requests)}",
                                 type="function", function=dict(name=name, arguments=json.dumps(arguments)))])
                    reason = "tool_calls"
                else:
                    delta = dict(role="assistant", content=reply[1])
                    reason = "stop"
                for chunk in [dict(delta=delta, finish_reason=None), dict(delta={}, finish_reason=reason)]:
                    data = dict(id="scripted", object="chat.completion.chunk", created=1, model="hermetic",
                                choices=[dict(index=0, **chunk)])
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
                contextWindow=65536, maxTokens=1024)])))

    def snapshot(self):
        with self.lock:
            return list(self.requests)

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
