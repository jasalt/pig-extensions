"""Real command-path savelast acceptance. Run: python3 test/integration/savelast.py."""
import os
from pathlib import Path
import tempfile

from rpc import ROOT, RPC, assistant, seed

EXT = Path(os.environ.get("SAVELAST_ROOT", ROOT / "extensions/savelast")).resolve()


def notification(rows, text, level):
    matches = [r for r in rows if r.get("type") == "extension_ui_request"
               and r.get("method") == "notify"]
    assert any(r.get("message", "").startswith(text) and r.get("notifyType") == level
               for r in matches), (text, level, rows)


def run():
    with tempfile.TemporaryDirectory(prefix="savelast-proof-") as temp:
        cwd = Path(temp)
        session = cwd / "session.jsonl"
        # A final non-message entry selects the active branch while the newest
        # assistant in the entire log belongs to the abandoned branch.
        exact = "  Résumé 決定\t\r\nlast  "
        seed(session, cwd, [assistant("old", None, [{"type": "text", "text": "OLD"}]),
                           assistant("active", "old", [{"type": "thinking", "thinking": "private"},
                                                      {"type": "text", "text": exact}]),
                           assistant("abandoned", "old", [{"type": "text", "text": "ABANDONED"}]),
                           dict(type="custom", id="leaf", parentId="active", customType="probe",
                                data={}, timestamp="2026-01-01T00:00:00.000Z")])
        with RPC(cwd, [EXT], session) as rpc:
            target = cwd / "notes/résumé with spaces.md"
            _, rows = rpc.call("prompt", message="/savelast notes/résumé with spaces.md")
            notification(rows, "Saved to: " + str(target), "info")
            assert target.read_bytes() == exact.encode(), target.read_bytes()
            # Shorter new contents overwrite existing bytes, no newline appended.
            target.write_text("old trailing contents" * 30)
            rpc.call("prompt", message="/savelast " + str(target))
            assert target.read_bytes() == exact.encode()
            _, rows = rpc.call("prompt", message="/savelast   ")
            notification(rows, "Saved to: ", "info")
            defaults = list(cwd.glob("[0-9]*.md"))
            assert len(defaults) == 1 and defaults[0].stem.isdigit(), defaults
            assert defaults[0].read_bytes() == exact.encode()
            blocker = cwd / "file-parent"
            blocker.write_text("file")
            _, rows = rpc.call("prompt", message="/savelast file-parent/out.md")
            notification(rows, "Failed to write file: ", "error")
            _, rows = rpc.call("prompt", message="/savelast " + str(cwd))
            notification(rows, "Failed to write file: ", "error")
            # Replacement must invalidate old branch data.
            rpc.call("new_session")
            _, rows = rpc.call("prompt", message="/savelast no-assistant.md")
            notification(rows, "No agent message found to save", "warning")
            assert not (cwd / "no-assistant.md").exists()
            switched = cwd / "switched"
            switched.mkdir()
            other = switched / "other.jsonl"
            seed(other, switched, [assistant("fresh", None, [{"type": "text", "text": "FRESH"}])])
            rpc.call("switch_session", sessionPath=str(other))
            _, rows = rpc.call("prompt", message="/savelast replacement.md")
            notification(rows, "Saved to: " + str(switched / "replacement.md"), "info")
            assert (switched / "replacement.md").read_bytes() == b"FRESH"
            assert not (cwd / "replacement.md").exists()
        for reason in ["error", "aborted"]:
            seed(session, cwd, [assistant("poisoned", None,
                                         [{"type": "text", "text": reason}], reason)])
            with RPC(cwd, [EXT], session) as rpc:
                rpc.call("prompt", message="/savelast " + reason + ".md")
                assert (cwd / (reason + ".md")).read_bytes() == reason.encode()
        seed(session, cwd, [assistant("old", None, [{"type": "text", "text": "must not save"}]),
                           assistant("textless", "old", [{"type": "thinking", "thinking": "private"}])])
        with RPC(cwd, [EXT], session) as rpc:
            _, rows = rpc.call("prompt", message="/savelast textless.md")
            notification(rows, "Last agent message has no text content to save", "warning")
            assert not (cwd / "textless.md").exists()
        with RPC(cwd, [EXT]) as rpc:
            _, rows = rpc.call("prompt", message="/savelast ephemeral.md")
            notification(rows, "No agent message found to save", "warning")
    print("PASS savelast real RPC: branch/bytes/default/overwrite/errors/replacement/poisoned/textless/ephemeral")


if __name__ == "__main__":
    run()
