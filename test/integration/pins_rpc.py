"""Real-host /pin commands, branch-local state and persistence over RPC.

Run: python3 -m test.integration.pins_rpc. No model or network is used.
"""
import json
from pathlib import Path
import tempfile

from .rpc import ROOT, RPC, assistant, seed

EXT = ROOT / "extensions/pins"
TS = "2026-01-01T00:00:00.000Z"


def text(value):
    return [{"type": "text", "text": value}]


def state(identifier, parent, data):
    return dict(type="custom", id=identifier, parentId=parent, customType="pin-state", data=data, timestamp=TS)


def notes(rows):
    return [(r.get("notifyType"), r.get("message", "")) for r in rows
            if r.get("type") == "extension_ui_request" and r.get("method") == "notify"]


def pin_states(session):
    rows = [json.loads(line) for line in Path(session).read_text().splitlines()]
    return [r["data"] for r in rows if r.get("customType") == "pin-state"]


def expect(rpc, command, level, message):
    _, rows = rpc.call("prompt", message=command)
    found = notes(rows)
    assert (level, message) in found, (command, found)
    return rows


def answer_select(rpc, command, choose):
    identifier = rpc.send("prompt", message=command)
    request, _ = rpc.wait_event("extension_ui_request")
    while request.get("method") != "select":
        request, _ = rpc.wait_event("extension_ui_request", since=len(rpc.events))
    options = request["options"]
    reply = dict(type="extension_ui_response", id=request["id"])
    if choose is None:
        reply["cancelled"] = True
    else:
        reply["value"] = choose(options)
    rpc.proc.stdin.write(json.dumps(reply) + "\n")
    rpc.proc.stdin.flush()
    _, rows = rpc.wait_response(identifier)
    return request, rows


def run():
    with tempfile.TemporaryDirectory(prefix="pins-rpc-") as temp:
        cwd = Path(temp)
        session = cwd / "session.jsonl"
        first = dict(id=1, label="First answer", text="# First answer\n\nbody", pinnedAt=1)
        abandoned = [dict(id=1, label="X", text="X", pinnedAt=1), dict(id=2, label="Y", text="Y", pinnedAt=2)]
        seed(session, cwd, [
            dict(type="message", id="u1", parentId=None, timestamp=TS,
                 message=dict(role="user", content="hello", timestamp=1)),
            assistant("a1", "u1", text("# First answer\n\nbody")),
            state("sx", "a1", dict(pins=abandoned, nextId=3)),
            assistant("a3", "sx", text("ABANDONED")),
            state("s1", "a1", dict(pins=[first], nextId=2)),
            assistant("a2", "s1", text("Second, active")),
            assistant("a4", "a2", [{"type": "thinking", "thinking": "only thinking"}]),
        ])
        with RPC(cwd, [EXT], session) as rpc:
            expect(rpc, "/pin rm 2", "error", "No pin #2")  # #2 exists only on the abandoned branch
            expect(rpc, "/pin", "info", '📌 Pinned as #2 "Second, active" — recall with /pin show 2')
            request, rows = answer_select(rpc, "/pin pick", lambda options: options[1])
            assert request["title"] == "Pin which message?", request
            assert request["options"] == ["2 back · Second, active", "3 back · # First answer body"], request
            assert ('info', '📌 Pinned as #3 "First answer" — recall with /pin show 3') in notes(rows), rows
            before = len(pin_states(session))
            _, rows = answer_select(rpc, "/pin pick", None)
            assert len(pin_states(session)) == before and not notes(rows), "cancelled pick must not pin"
            expect(rpc, "/pin   my   label  ", "info", '📌 Pinned as #4 "my   label" — recall with /pin show 4')
            expect(rpc, "/pin list of plugins", "info", '📌 Pinned as #5 "list of plugins" — recall with /pin show 5')
            last = pin_states(session)[-1]
            assert last["nextId"] == 6 and [p["id"] for p in last["pins"]] == [1, 2, 3, 4, 5], last
            assert last["pins"][2]["text"] == "# First answer\n\nbody", last
            assert all(isinstance(p["pinnedAt"], int) and p["pinnedAt"] > 1_700_000_000_000 for p in last["pins"][1:]), last
            expect(rpc, "/pin rm 1", "info", 'Removed pin #1 "First answer"')
            expect(rpc, "/pin RM", "error", "Usage: /pin rm <n>")
            expect(rpc, "/pin rm nope", "error", "No pin #nope")
            expect(rpc, "/pin show 9", "error", "No pin #9")
            expect(rpc, "/pin show", "warning", "/pin viewer requires interactive TUI mode")
            expect(rpc, "/pin list 2", "warning", "/pin viewer requires interactive TUI mode")
            _, rows = rpc.call("prompt", message="/pin help")
            found = notes(rows)
            assert len(found) == 1 and found[0][0] == "info", found
            assert found[0][1].startswith("/pin [label]    Pin the latest nonempty assistant text"), found
            assert pin_states(session)[-1]["nextId"] == 6
        # A restarted host restores the same branch snapshot.
        with RPC(cwd, [EXT], session) as rpc:
            expect(rpc, "/pin rm 1", "error", "No pin #1")
            expect(rpc, "/pin rm 2", "info", 'Removed pin #2 "Second, active"')
            expect(rpc, "/pin clear", "info", "All pins removed")
            assert pin_states(session)[-1] == dict(pins=[], nextId=1)
            expect(rpc, "/pin again", "info", '📌 Pinned as #1 "again" — recall with /pin show 1')
            rpc.call("new_session")
            expect(rpc, "/pin", "warning", "No assistant message to pin")
            expect(rpc, "/pin pick", "warning", "No assistant messages to pin")

        corrupt = cwd / "corrupt.jsonl"
        seed(corrupt, cwd, [assistant("c1", None, text("answer")),
                            state("cs", "c1", dict(pins=[dict(id=1, label="x", pinnedAt=1)], nextId=2))])
        with RPC(cwd, [EXT], corrupt) as rpc:
            message = "pins: Invalid saved pin-state; no pins were changed"
            for command in ["/pin", "/pin rm 1", "/pin show", "/pin pick"]:
                expect(rpc, command, "error", message)
            assert len(pin_states(corrupt)) == 1, "corrupt state must not be overwritten"
            expect(rpc, "/pin clear", "info", "All pins removed")
            assert pin_states(corrupt)[-1] == dict(pins=[], nextId=1)
    print("PASS /pin real host: branch-local restore, pin/pick/cancel/labels/rm/show/help/clear, restart persistence, corrupt-state refusal")


if __name__ == "__main__":
    run()
