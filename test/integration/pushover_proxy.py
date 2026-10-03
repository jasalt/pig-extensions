"""Local-only TLS proxy: production fixed Pushover endpoint, no live traffic."""
import ssl
import socketserver
import subprocess
import threading
from pathlib import Path
from urllib.parse import parse_qs


class PushoverProxy:
    def __init__(self, directory):
        directory = Path(directory)
        cert = directory / "certificate.pem"
        key = directory / "key.pem"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", str(key), "-out", str(cert), "-days", "1",
                        "-subj", "/CN=api.pushover.net", "-addext",
                        "subjectAltName=DNS:api.pushover.net"],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        key.chmod(0o600)
        cert.chmod(0o600)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        self.requests = []
        self.status = 200
        self.body = b'{"status":1}'
        self.hold = False
        self.started = threading.Event()
        self.disconnected = threading.Event()
        owner = self

        class Handler(socketserver.BaseRequestHandler):
            def handle(self):
                connection = self.request
                connection.settimeout(10)
                header = b""
                while not header.endswith(b"\r\n\r\n"):
                    chunk = connection.recv(1)
                    if not chunk:
                        return
                    header += chunk
                assert header.startswith(b"CONNECT api.pushover.net:443 HTTP/1.1"), header
                connection.sendall(b"HTTP/1.1 200 Connection Established\r\n\r\n")
                with context.wrap_socket(connection, server_side=True) as tls:
                    stream = tls.makefile("rb")
                    request = stream.readline()
                    assert request == b"POST /1/messages.json HTTP/1.1\r\n", request
                    headers = {}
                    while True:
                        line = stream.readline()
                        if line == b"\r\n":
                            break
                        name, value = line.decode().split(":", 1)
                        headers[name.lower()] = value.strip()
                    payload = stream.read(int(headers["content-length"]))
                    owner.requests.append(parse_qs(payload.decode(), keep_blank_values=True))
                    owner.started.set()
                    if owner.hold:
                        assert tls.recv(1) == b"", "unexpected request after pending POST"
                        owner.disconnected.set()
                        stream.close()
                        return
                    body = owner.body
                    response = (f"HTTP/1.1 {owner.status} Fixture\r\n"
                                f"Content-Length: {len(body)}\r\nConnection: close\r\n"
                                "Location: https://api.pushover.net/must-not-follow\r\n\r\n").encode()
                    tls.sendall(response + body)
                    stream.close()

        class Server(socketserver.ThreadingTCPServer):
            allow_reuse_address = True
            daemon_threads = True

        self.server = Server(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.env = {"HTTPS_PROXY": f"http://127.0.0.1:{self.server.server_address[1]}",
                    "SSL_CERT_FILE": str(cert), "NO_PROXY": ""}

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
