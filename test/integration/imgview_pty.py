"""Interactive /imgcat renderer through the real host TUI.

Run: python3 -m test.integration.imgview_pty. Proves the registered renderer
reaches the terminal stream (label, fallback text or protocol bytes). It does
not prove a graphical terminal displayed an image.
"""
import base64
from pathlib import Path
import tempfile

from .imgview import EXT, png
from .pty_host import PTYHost
from .rpc import seed


def run():
    image = png(2, 2)
    encoded = base64.b64encode(image).decode()
    for name, env, expectation in [
        ("fallback", {}, "text"),
        ("kitty", dict(TERM="xterm-kitty", TERM_PROGRAM="kitty"), "kitty"),
        ("explicit-off", dict(TERM="xterm-kitty", TERM_PROGRAM="kitty", PI_IMAGE_PROTOCOL="none"), "text"),
    ]:
        with tempfile.TemporaryDirectory(prefix=f"imgview-pty-{name}-") as temp:
            cwd = Path(temp)
            (cwd / "dot.png").write_bytes(image)
            session = cwd / "session.jsonl"
            seed(session, cwd, [])
            with PTYHost(cwd, [EXT], session, env=env) as host:
                host.wait_command("imgcat")
                mark = host.command("/imgcat dot.png")
                host.pump_until(lambda: bool(host.entries("imgview-image")))
                host.pump_until(lambda: b"imgview: " in host.raw[mark:] and b"(image/png," in host.raw[mark:])
                host.settle()
                out = bytes(host.raw[mark:])
                kitty = b"\x1b_Ga=T" in out and encoded.encode()[:32] in out
                fallback = b"[image/png]" in out and b"2x2" in out
                if expectation == "kitty":
                    assert kitty and not fallback, out[-3000:]
                else:
                    assert fallback and b"\x1b_G" not in out, out[-3000:]
                print(f"PASS /imgcat interactive renderer ({name}): {'kitty protocol bytes' if kitty else 'native fallback text'}")


if __name__ == "__main__":
    run()
