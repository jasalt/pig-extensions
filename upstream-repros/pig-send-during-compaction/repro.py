"""Interactive mode: an extension's sendUserMessage during compaction
returns no error to the extension. PIG_BIN=... python3 repro.py"""
import json
from pathlib import Path
import re
import sys
import tempfile
import time

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
from harness import PTY, ScriptedModel, result, text_of  # noqa: E402

HERE = Path(__file__).resolve().parent
ANSI = re.compile(rb"\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)?")


def script(messages):
    if any("summar" in text_of(m).lower() for m in messages[-1:]):
        time.sleep(3)  # keep compaction in progress while the extension sends
    return "model reply " + "x" * 400


with tempfile.TemporaryDirectory() as home:
    model = ScriptedModel(home, script)
    (Path(home) / "agent/settings.json").write_text(json.dumps(dict(
        quietStartup=True, compaction=dict(keepRecentTokens=1))))
    try:
        with PTY(home, [HERE], args=["--model", model.model]) as term:
            term.until(lambda: b"\x1b[?2004h" in term.raw)
            term.wait_command("compact-probe", "send a message during compaction")
            for turn in range(3):
                count = len(model.requests)
                term.write(f"turn {turn} {'y' * 300}\r")
                term.until(lambda: len(model.requests) > count, 30)
                term.settle(1)
            mark = len(term.raw)
            term.write("/compact-probe\r")
            term.until(lambda: b"sendUserMessage returned" in term.raw[mark:] or b"compaction did not" in term.raw[mark:], 30)
            term.settle(8, 30)  # compaction completes; a queued message would run now
            screen = ANSI.sub(b"", bytes(term.raw[mark:])).decode(errors="replace")
            returned = re.findall(r"sendUserMessage returned err=\S+|compaction (?:failed|did not)[^\n]*", screen)
            host_error = "compaction is in progress" in screen
            delivered = any("MESSAGE-SENT-DURING-COMPACTION" in text_of(m)
                            for r in model.requests for m in r["messages"])
            result("pig-send-during-compaction",
                   bool(returned) and returned[0] == "sendUserMessage returned err=<nil>" and not delivered,
                   f"extension saw: {sorted(set(returned))}; host showed compaction error: {host_error}; "
                   f"message reached the model: {delivered}")
    finally:
        model.close()
