"""Claude → PiG real-host import, reopen and hermetic continuation.
Run: python3 -m test.integration.session_migrate
Also runs unchanged with PIG_BIN=<fused binary> PIG_FUSED=1.
"""
import hashlib
import json
from pathlib import Path
import tempfile

from .rpc import ROOT, RPC, seed
from .scripted_provider import ScriptedProvider, text_of

EXT = ROOT / "extensions/session-migrate"
FIXTURE = EXT / "testdata/claude-native.jsonl"


def rows(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def note(rpc, response_rows, prefix):
    errors = [r for r in response_rows if r.get("type") == "extension_error"]
    assert not errors, errors
    for row in response_rows:
        if row.get("method") == "notify" and row.get("message", "").startswith(prefix):
            return row["message"]
    while True:
        row, _ = rpc.wait_event("extension_ui_request", since=len(rpc.events))
        assert row.get("type") != "extension_error", row
        if row.get("method") == "notify" and row.get("message", "").startswith(prefix):
            return row["message"]


def run():
    with tempfile.TemporaryDirectory(prefix="session-migrate-proof-") as temp:
        cwd = Path(temp)
        source = cwd / "Claude session with spaces.jsonl"
        source.write_bytes(FIXTURE.read_bytes())
        before = hashlib.sha256(source.read_bytes()).hexdigest()
        session_dir = cwd / "sessions"
        with ScriptedProvider(lambda messages: ("text", "CONTINUED_IMPORTED_HISTORY")) as provider:
            agent = cwd / "agent"
            agent.mkdir()
            (agent / "models.json").write_text(json.dumps(provider.config))
            starter = cwd / "starter.jsonl"
            seed(starter, cwd, [])
            with RPC(cwd, [EXT], session=starter, model=provider.model,
                     extra_args=["--session-dir", str(session_dir)]) as rpc:
                _, seen = rpc.call("prompt", message=f"/session-migrate inspect claude {source}")
                report = json.loads(note(rpc, seen, '{"sourceFormat"'))
                assert report["preserved"]["tool_calls"] == 3, report
                assert not list(session_dir.glob("*.jsonl")), "inspection wrote a session"
                _, seen = rpc.call("prompt", message=f"/session-migrate import claude {source}")
                saved = note(rpc, seen, "Saved imported session: ")
                imported = Path(saved.split("; manifest: ")[0].removeprefix("Saved imported session: "))
                # Command response is the barrier for the awaited replacement.
                state, _ = rpc.call("get_state")
                assert state["data"]["sessionFile"] == str(imported), state
                assert state["data"]["sessionName"] == "repair-event-window-boundary", state
                original = rows(imported)
                audit = json.loads(Path(str(imported) + ".migration.json").read_text())
                assert audit["source"]["sourceSHA256"] == before
                initial_count = 2 + audit["source"]["preserved"]["messages"] + audit["source"]["preserved"].get("compactions", 0)
                initial_bytes = b"".join(imported.read_bytes().splitlines(keepends=True)[:initial_count])
                # PiG may append model/thinking metadata while opening a file.
                assert audit["initialTargetSHA256"] == hashlib.sha256(initial_bytes).hexdigest()
                messages, _ = rpc.call("get_messages")
                history = messages["data"]["messages"]
                assert [m["role"] for m in history].count("toolResult") == 3, history
                assert len([b for m in history for b in (m["content"] if isinstance(m["content"], list) else [])
                            if b.get("type") == "image"]) == 1
                assert not provider.snapshot(), "import unexpectedly called a model"
                rpc.call("prompt", message="Continue using the imported conversation.")
                rpc.wait_event("agent_end")
                rpc.call("get_state")
                requests = provider.snapshot()
                assert len(requests) == 1, requests
                sent = requests[0]["messages"]
                assert len([m for m in sent if m["role"] == "tool"]) == 3, sent
                assert any("7319" in text_of(m) for m in sent), "native Claude conversation not sent to provider"
                assert rows(imported)[:len(original)] == original, "continuation rewrote imported history"
                assert any("CONTINUED_IMPORTED_HISTORY" in text_of(r.get("message", {})) for r in rows(imported))
            with RPC(cwd, [EXT], session=imported, model=provider.model) as rpc:
                messages, _ = rpc.call("get_messages")
                assert any("CONTINUED_IMPORTED_HISTORY" in text_of(m) for m in messages["data"]["messages"])
                _, seen = rpc.call("prompt", message=f"/session-migrate save claude {source}")
                saved = note(rpc, seen, "Saved imported session: ")
                assert str(imported) not in saved
                state, _ = rpc.call("get_state")
                assert state["data"]["sessionFile"] == str(imported), "save switched sessions"

                broken = cwd / "broken.jsonl"
                broken.write_text(json.dumps(dict(type="user", uuid="bad", parentUuid="missing",
                                                 message=dict(role="user", content="PRIVATE_BAD_BODY"))) + "\n")
                existing = set(cwd.rglob("*.migration.json"))
                _, seen = rpc.call("prompt", message=f"/session-migrate import claude {broken}")
                errors = [r for r in seen if r.get("type") == "extension_error"]
                assert errors and "missing UUID" in errors[0]["error"], seen
                assert "PRIVATE_BAD_BODY" not in json.dumps(seen)
                assert set(cwd.rglob("*.migration.json")) == existing
                state, _ = rpc.call("get_state")
                assert state["data"]["sessionFile"] == str(imported)

                compacted = cwd / "compacted.jsonl"
                compacted.write_text("".join(json.dumps(r) + "\n" for r in [
                    dict(type="user", uuid="old", message=dict(role="user", content="PRE_COMPACTION_PRIVATE")),
                    dict(type="system", subtype="compact_boundary", uuid="boundary", logicalParentUuid="old"),
                    dict(type="user", uuid="summary", parentUuid="boundary", isCompactSummary=True,
                         message=dict(role="user", content="PORTABLE_COMPACTION_SUMMARY")),
                    dict(type="user", uuid="new", parentUuid="summary",
                         message=dict(role="user", content="POST_COMPACTION_TURN")),
                ]))
                _, seen = rpc.call("prompt", message=f"/session-migrate import claude {compacted}")
                note(rpc, seen, "Saved imported session: ")
                cursor = len(rpc.events)
                rpc.call("prompt", message="Continue after compaction.")
                rpc.wait_event("agent_end", since=cursor)
                rpc.call("get_state")
                sent = json.dumps(provider.snapshot()[-1]["messages"])
                assert "PORTABLE_COMPACTION_SUMMARY" in sent and "POST_COMPACTION_TURN" in sent, sent
                assert "PRE_COMPACTION_PRIVATE" not in sent, "pre-compaction history replayed"

                detached = cwd / "detached-result.jsonl"
                detached.write_text("".join(json.dumps(r) + "\n" for r in [
                    dict(type="user", uuid="u", sessionId="synthetic", message=dict(role="user", content="Read a fixture")),
                    dict(type="assistant", uuid="a", parentUuid="u", sessionId="synthetic",
                         message=dict(role="assistant", content=[dict(type="tool_use", id="call", name="Read", input={})])),
                    dict(type="user", uuid="r", parentUuid="a", sourceToolAssistantUUID="a", sessionId="synthetic",
                         message=dict(role="user", content=[dict(type="tool_result", tool_use_id="call", content="DETACHED_RESULT_MARKER")])),
                    dict(type="assistant", uuid="next", parentUuid="a", sessionId="synthetic",
                         message=dict(role="assistant", content="Continued from assistant, not result")),
                    dict(type="last-prompt", leafUuid="next"),
                ]))
                detached_hash = hashlib.sha256(detached.read_bytes()).hexdigest()
                _, seen = rpc.call("prompt", message=f"/session-migrate import claude {detached}")
                note(rpc, seen, "Saved imported session: ")
                messages, _ = rpc.call("get_messages")
                assert [m["role"] for m in messages["data"]["messages"]] == ["user", "assistant", "toolResult", "assistant"]
                cursor = len(rpc.events)
                rpc.call("prompt", message="Continue after detached tool result recovery.")
                rpc.wait_event("agent_end", since=cursor)
                rpc.call("get_state")
                sent = provider.snapshot()[-1]["messages"]
                assert len([m for m in sent if m["role"] == "tool"]) == 1, sent
                assert "DETACHED_RESULT_MARKER" in json.dumps(sent), sent
                assert hashlib.sha256(detached.read_bytes()).hexdigest() == detached_hash
        assert hashlib.sha256(source.read_bytes()).hexdigest() == before, "source modified"
        for path in session_dir.glob("*.jsonl*"):
            assert path.stat().st_mode & 0o777 == 0o600, path
    print("PASS Claude native fixture → PiG: inspect/import/switch/title/tools/image/manifest/continuation/reopen/save/malformed-refusal/compaction/detached-result/source unchanged")


if __name__ == "__main__":
    run()
