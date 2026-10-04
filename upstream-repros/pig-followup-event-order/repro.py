"""Observation: a follow-up queued in the same run gets message_start but no
before_agent_start, and the run settles once. PIG_BIN=... python3 repro.py"""
from pathlib import Path
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import RPC, ScriptedModel  # noqa: E402

HERE = Path(__file__).resolve().parent

with tempfile.TemporaryDirectory() as home:
    model = ScriptedModel(home, lambda messages: "reply")
    try:
        with RPC(home, [HERE], model=model.model) as rpc:
            mark = len(rpc.events)
            rpc.call("prompt", message="/send-two")
            rpc.until(lambda r: r.get("type") == "agent_settled", since=mark)
            time.sleep(1)
            rpc.call("prompt", message="/events")
            events = next(n for n in rpc.notifications() if n.startswith("events "))[len("events "):].split(" | ")
            expected = ["before_agent_start:ONE", "message_start:ONE", "message_start:TWO", "agent_settled"]
            print(f"pig-followup-event-order: {'OBSERVED' if events == expected else 'DIFFERENT'} — {events}")
    finally:
        model.close()
