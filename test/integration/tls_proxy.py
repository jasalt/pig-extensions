"""Private local TLS proxy shared by two production-endpoint test callers."""
import ssl
import socketserver
import subprocess
import threading
from pathlib import Path


class TLSProxy:
    def __init__(self, directory, hostname):
        directory = Path(directory)
        cert = directory / "certificate.pem"
        key = directory / "key.pem"
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes",
                        "-keyout", str(key), "-out", str(cert), "-days", "1",
                        "-subj", "/CN=" + hostname, "-addext",
                        "subjectAltName=DNS:" + hostname],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        key.chmod(0o600)
        cert.chmod(0o600)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert, key)
        self.requests = []
        self.status = 200
        self.body = b"{}"
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
                assert header.startswith(f"CONNECT {hostname}:443 HTTP/1.1".encode()), header
                connection.sendall(b"HTTP/1.1 200 Connection Established\r\n\r\n")
                with context.wrap_socket(connection, server_side=True) as tls:
                    stream = tls.makefile("rb")
                    method, target, protocol = stream.readline().decode().strip().split()
                    assert protocol == "HTTP/1.1", protocol
                    headers = {}
                    while True:
                        line = stream.readline()
                        if line == b"\r\n":
                            break
                        name, value = line.decode().split(":", 1)
                        headers[name.lower()] = value.strip()
                    payload = stream.read(int(headers.get("content-length", "0")))
                    status, body = owner.response(method, target, headers, payload)
                    hold = owner.hold
                    owner.started.set()
                    if hold:
                        assert tls.recv(1) == b"", "unexpected data after held request"
                        owner.disconnected.set()
                        stream.close()
                        return
                    response = (f"HTTP/1.1 {status} Fixture\r\n"
                                f"Content-Length: {len(body)}\r\nConnection: close\r\n"
                                f"Location: https://{hostname}/must-not-follow\r\n\r\n").encode()
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

    def response(self, method, target, headers, payload):
        raise NotImplementedError

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=5)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
