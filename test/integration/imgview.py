"""Real-host imgview commands and show_image tool. Run: python3 -m test.integration.imgview.

A fake xdg-open on PATH records argv; no browser is launched. A hermetic local
model drives the tool. No network is used beyond 127.0.0.1.
"""
import base64
import json
import os
from pathlib import Path
import stat
import struct
import tempfile
import time
import zlib

from .rpc import ROOT, RPC
from .tool_provider import ToolProvider

EXT = ROOT / "extensions/imgview"


def png(width, height):
    def chunk(kind, data):
        return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
    rows = b"".join(b"\x00" + b"\xff\x00\x00" * width for _ in range(height))
    return (b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0))
            + chunk(b"IDAT", zlib.compress(rows)) + chunk(b"IEND", b""))


def opened_lines(record, count, timeout=10):
    """The opener is started asynchronously; wait for its argv record."""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if record.exists():
            lines = record.read_text().splitlines()
            if len(lines) >= count:
                return lines
        time.sleep(0.02)
    raise AssertionError(f"opener did not record {count} launch(es)")


def notifications(rows):
    return [(r.get("notifyType"), r.get("message", "")) for r in rows
            if r.get("type") == "extension_ui_request" and r.get("method") == "notify"]


def custom_messages(session):
    rows = [json.loads(line) for line in Path(session).read_text().splitlines()]
    return [r for r in rows if r.get("type") == "custom_message" and r.get("customType") == "imgview-image"]


def fake_opener(directory, record):
    bin_dir = directory / "fakebin"
    bin_dir.mkdir()
    script = bin_dir / "xdg-open"
    script.write_text(f"#!/bin/sh\nprintf '%s\\n' \"$@\" >> {record}\n")
    script.chmod(0o755)
    return bin_dir


def environment(cwd, opener_dir=None):
    path = "/usr/bin:/bin"
    if opener_dir:
        path = f"{opener_dir}:{path}"
    tmp = cwd / "tmp"
    tmp.mkdir(exist_ok=True)
    # Keep PiG's Go toolchain reachable for building the extension.
    go_dirs = {str(Path(p).parent) for p in [os.popen("command -v go").read().strip()] if p}
    return dict(PATH=":".join([path, *go_dirs]), TMPDIR=str(tmp))


def commands():
    with tempfile.TemporaryDirectory(prefix="imgview-commands-") as temp:
        cwd = Path(temp)
        image = png(3, 2)
        (cwd / "pics with space").mkdir()
        (cwd / "pics with space/red.png").write_bytes(image)
        (cwd / "notes.txt").write_text("plain text")
        record = cwd / "opened.txt"
        opener = fake_opener(cwd, record)
        session = cwd / "session.jsonl"
        from .rpc import seed
        seed(session, cwd, [])
        with RPC(cwd, [EXT], session, extra_env=environment(cwd, opener)) as rpc:
            _, rows = rpc.call("prompt", message="/imgcat pics with space/red.png")
            target = cwd / "pics with space/red.png"
            assert ("info", f"imgview: {target}\nmime=image/png bytes={len(image)} mode=terminal") in notifications(rows), rows
            assert not any(r.get("type") == "agent_start" for r in rows), "imgcat must not start a turn"
            saved = custom_messages(session)
            assert len(saved) == 1, saved
            message = saved[0]
            assert message["content"] == f"imgview: {target} (image/png, {len(image)} bytes)"
            assert message["display"] is True
            details = message["details"]
            assert details["image"] == dict(data=base64.b64encode(image).decode(), mimeType="image/png")
            assert details["resolved"] == str(target) and details["bytes"] == len(image)
            assert not record.exists(), "terminal mode must not open a browser"

            _, rows = rpc.call("prompt", message="/imgshow pics with space/red.png")
            opened = opened_lines(record, 1)
            assert len(opened) == 1, opened
            viewer = Path(opened[0])
            assert viewer.parent == cwd / "tmp/pig-imgview", viewer
            assert stat.S_IMODE(viewer.stat().st_mode) == 0o600
            assert stat.S_IMODE(viewer.parent.stat().st_mode) == 0o700
            html = viewer.read_text()
            assert "data:image/png;base64," + base64.b64encode(image).decode() in html
            assert ("info", f"imgview: {target}\nmime=image/png bytes={len(image)} mode=browser\nbrowser={viewer}") in notifications(rows), rows
            assert len(custom_messages(session)) == 1, "browser-only must not add a transcript image"

            data_uri = "data:image/png;base64," + base64.b64encode(image).decode()
            _, rows = rpc.call("prompt", message="/imgboth " + data_uri)
            assert len(opened_lines(record, 2)) == 2
            assert len(custom_messages(session)) == 2
            assert custom_messages(session)[-1]["details"]["resolved"] == "<data uri>"

            for command, level, prefix in [
                ("/imgcat   ", "warning", "Usage: /imgcat <path|url|data:uri>"),
                ("/imgshow", "warning", "Usage: /imgshow <path|url|data:uri>"),
                ("/imgcat notes.txt", "error", f"imgview: {cwd / 'notes.txt'} has unsupported MIME application/octet-stream."),
                ("/imgcat missing.png", "error", f"imgview: cannot read {cwd / 'missing.png'}: no such file or directory"),
                ("/imgcat pics with space", "error", f"imgview: {cwd / 'pics with space'} is not a regular file"),
                ("/imgcat data:nope", "error", "imgview: malformed data: URI"),
            ]:
                _, rows = rpc.call("prompt", message=command)
                assert any(n == (level, prefix) for n in notifications(rows)), (command, notifications(rows))
            assert len(custom_messages(session)) == 2

        # No opener on PATH: browser-only reports only the error; both keeps
        # the inline image and omits the browser claim.
        with RPC(cwd, [EXT], session, extra_env=environment(cwd)) as rpc:
            _, rows = rpc.call("prompt", message="/imgshow pics with space/red.png")
            found = notifications(rows)
            assert len(found) == 1 and found[0][0] == "error" and found[0][1].startswith("imgview: failed to open browser: "), found
            _, rows = rpc.call("prompt", message="/imgboth pics with space/red.png")
            found = notifications(rows)
            assert found[0][0] == "error" and found[0][1].startswith("imgview: failed to open browser: "), found
            assert found[1] == ("info", f"imgview: {target}\nmime=image/png bytes={len(image)} mode=both"), found
            assert len(custom_messages(session)) == 3
        print("PASS imgview commands: custom message persistence/no turn, viewer privacy/argv, data URI, errors, missing opener")


def tool_case(cwd, arguments, opener=True):
    record = cwd / "opened.txt"
    if record.exists():
        record.unlink()
    opener_dir = cwd / "fakebin" if opener else None
    with ToolProvider("show_image", arguments) as provider:
        provider.config["providers"]["fixture"]["models"][0]["input"] = ["text", "image"]
        (cwd / "agent").mkdir(exist_ok=True)
        (cwd / "agent/models.json").write_text(json.dumps(provider.config))
        with RPC(cwd, [EXT], extra_env=environment(cwd, opener_dir), model=provider.model) as rpc:
            rpc.call("prompt", message="Show the fixture image.")
            _, rows = rpc.wait_event("agent_end")
        updates = [r for r in rows if r.get("type") == "tool_execution_update"]
        ends = [r for r in rows if r.get("type") == "tool_execution_end"]
        assert len(ends) == 1, rows
        return ends[0], updates, provider.requests, record


def tools():
    with tempfile.TemporaryDirectory(prefix="imgview-tool-") as temp:
        cwd = Path(temp)
        image = png(4, 4)
        (cwd / "plot.png").write_bytes(image)
        (cwd / "plain.txt").write_text("text")
        fake_opener(cwd, cwd / "opened.txt")
        encoded = base64.b64encode(image).decode()

        end, updates, requests, record = tool_case(cwd, dict(source="plot.png", caption="fixture plot"))
        result = end["result"]
        assert not end.get("isError"), end
        assert updates and updates[0]["partialResult"]["content"][0]["text"] == "Loading plot.png...", updates
        content = result["content"]
        assert content[0]["type"] == "text" and content[1] == dict(type="image", data=encoded, mimeType="image/png"), content
        text = content[0]["text"]
        assert text.startswith(f"Showed {cwd / 'plot.png'} (image/png, {len(image)} bytes) via mode=terminal.\nCaption: fixture plot"), text
        assert result["details"]["mode"] == "terminal" and "browserPath" not in result["details"], result
        assert not record.exists()
        assert encoded in json.dumps(requests[-1]), "image did not reach the model request"

        end, _, requests, record = tool_case(cwd, dict(source="plot.png", mode="browser"))
        result = end["result"]
        assert not end.get("isError"), end
        assert all(block["type"] == "text" for block in result["content"]), result
        viewer = opened_lines(record, 1)[0]
        assert f"Browser: opened {viewer}." in result["content"][0]["text"], result
        assert result["details"]["browserPath"] == viewer and result["details"]["openCommand"] == "xdg-open " + viewer
        assert encoded not in json.dumps(requests[-1]), "browser mode must not send image base64 to the model"

        end, _, _, _ = tool_case(cwd, dict(source="plot.png", mode="browser"), opener=False)
        assert end.get("isError"), end
        text = end["result"]["content"][0]["text"]
        assert "Browser open failed: " in text and "Browser: opened" not in text, text

        end, _, _, _ = tool_case(cwd, dict(source="plot.png", mode="both"), opener=False)
        assert not end.get("isError"), end
        assert end["result"]["content"][1]["type"] == "image" and "Browser open failed: " in end["result"]["content"][0]["text"]

        end, _, _, _ = tool_case(cwd, dict(source="plain.txt"))
        assert end.get("isError"), end
        assert f"Refusing to show {cwd / 'plain.txt'}: detected MIME application/octet-stream" in json.dumps(end["result"]), end

        end, _, _, _ = tool_case(cwd, dict(source="missing.png"))
        assert end.get("isError") and "show_image failed: cannot read" in json.dumps(end["result"]), end
        print("PASS show_image real host: ordered text/image result, model delivery, browser opt-in, honest launch errors, rejection")


if __name__ == "__main__":
    commands()
    tools()
