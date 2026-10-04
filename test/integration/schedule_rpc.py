"""Real-host scheduler proofs over RPC. Run: python3 -m test.integration.schedule_rpc.

A scripted local model drives agent turns; nothing leaves 127.0.0.1. Stores,
trust and ledgers live under a temporary PIG_HOME.
"""
from datetime import datetime, timedelta, timezone
import json
import os
from pathlib import Path
import queue
import tempfile
import time

from .rpc import ROOT, RPC
from .scripted_provider import ScriptedProvider, text_of

EXT = ROOT / "extensions/schedule"
SECRET = "ghp_" + "01" * 18


def iso(dt):
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def job(identifier, **fields):
    now = datetime.now(timezone.utc)
    row = dict(id=identifier, name=identifier, prompt="Report the weather.", action="prompt",
               schedule=dict(type="interval", everyMs=3_600_000, every="1h"), scope="global", enabled=True,
               missedWindow="catch_up_one", tier="read_only", terminated=None, createdAt=iso(now), updatedAt=iso(now),
               lastRunAt=None, nextRunAt=iso(now - timedelta(seconds=5)), runCount=0, lastStatus=None)
    row.update(fields)
    return row


def write_store(path, jobs):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(dict(version=1, jobs=jobs)))


def read_store(path):
    return {row["id"]: row for row in json.loads(path.read_text())["jobs"]}


def wait_for(predicate, timeout=30, what="condition"):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        value = predicate()
        if value:
            return value
        time.sleep(0.05)
    raise AssertionError(f"timed out waiting for {what}")


def setup(cwd, provider=None):
    agent = cwd / "agent"
    agent.mkdir(exist_ok=True)
    if provider:
        (agent / "models.json").write_text(json.dumps(provider.config))
    state = cwd / "home/state/schedule"
    return state


def pump(rpc):
    """Move already-received host frames into rpc.events without blocking."""
    while True:
        try:
            rpc.events.append(rpc.lines.get_nowait())
        except queue.Empty:
            return rpc.events


def notes(rows):
    return [(r.get("notifyType"), r.get("message", "")) for r in rows
            if r.get("type") == "extension_ui_request" and r.get("method") == "notify"]


def privilege_and_ownership():
    """A read_only scheduled turn is blocked from bash and terminated; the
    user's own next turn is not restricted."""
    with tempfile.TemporaryDirectory(prefix="schedule-privilege-") as temp:
        cwd = Path(temp)
        marker = cwd / "pwned"
        user_marker = cwd / "user-ran"

        def script(messages):
            last = messages[-1]
            if last.get("role") == "tool":
                return ("text", "done")
            text = text_of(last)
            if "[scheduled-task]" in text:
                return ("tool", "bash", dict(command=f"touch {marker}"))
            return ("tool", "bash", dict(command=f"touch {user_marker}"))

        with ScriptedProvider(script) as provider:
            state = setup(cwd, provider)
            write_store(state / "schedules.json", [job("weather")])
            with RPC(cwd, [EXT], model=provider.model) as rpc:
                time.sleep(2)
                assert not provider.snapshot(), "startup wave must be deferred to the first idle tick"
                assert read_store(state / "schedules.json")["weather"]["runCount"] == 0
                mark = len(rpc.events)
                rpc.call("new_session")
                _, rows = rpc.wait_event("agent_end", since=mark)
                ends = [r for r in rpc.events[mark:] if r.get("type") == "tool_execution_end"]
                assert len(ends) == 1 and ends[0]["isError"], ends
                assert "[pig-schedule] blocked bash: active scheduled job is tier=read_only" in json.dumps(ends[0]["result"]), ends[0]
                assert not marker.exists(), "blocked tool ran"
                time.sleep(0.5)
                assert len(provider.snapshot()) == 1, "a fully blocked batch must terminate the turn"
                row = wait_for(lambda: (r := read_store(state / "schedules.json")["weather"])["runCount"] == 1 and r, what="advance")
                assert row["lastStatus"] == "ok" and row["nextRunAt"] > iso(datetime.now(timezone.utc)), row
                runs = [json.loads(l) for l in (state / "runs.jsonl").read_text().splitlines()]
                assert runs[-1]["status"] == "delivered" and runs[-1]["source"] == "session_start", runs

                mark = len(rpc.events)
                rpc.call("prompt", message="Please touch the user marker.")
                rpc.wait_event("agent_end", since=mark)
                ends = [r for r in rpc.events[mark:] if r.get("type") == "tool_execution_end"]
                assert len(ends) == 1 and not ends[0]["isError"], ends
                wait_for(user_marker.exists, what="user bash")
                assert not marker.exists()
    print("PASS schedule real host: deferred startup wave, /new fires, read_only blocks bash with terminate, user turn unrestricted, store/ledger advance")


def queued_tiers():
    """Two jobs due together: the mutate prompt is queued as a follow-up in the
    read_only prompt's agent run. Each turn must get exactly its own tier."""
    with tempfile.TemporaryDirectory(prefix="schedule-queued-") as temp:
        cwd = Path(temp)
        readonly_marker, mutate_marker = cwd / "readonly-ran", cwd / "mutate-ran"

        def script(messages):
            last = messages[-1]
            if last.get("role") == "tool":
                return ("text", "done")
            text = text_of(last)
            target = mutate_marker if "tier: mutate" in text else readonly_marker
            return ("tool", "bash", dict(command=f"touch {target}"))

        with ScriptedProvider(script) as provider:
            state = setup(cwd, provider)
            write_store(state / "schedules.json", [job("first-readonly"), job("second-mutate", tier="mutate")])
            with RPC(cwd, [EXT], model=provider.model) as rpc:
                mark = len(rpc.events)
                rpc.call("new_session")
                wait_for(lambda: mutate_marker.exists() or None, what="mutate follow-up bash")
                wait_for(lambda: [r for r in pump(rpc)[mark:] if r.get("type") == "agent_end"], what="run end")
                ends = [r for r in rpc.events[mark:] if r.get("type") == "tool_execution_end"]
                assert [e["isError"] for e in ends] == [True, False], ends
                assert not readonly_marker.exists(), "read_only turn inherited the queued mutate tier"
                users = [text_of(m) for m in provider.snapshot()[-1]["messages"] if m.get("role") == "user"]
                assert sum("[scheduled-task]" in u for u in users) == 2, users
    print("PASS schedule real host: queued follow-up tiers own only their own turn (read_only blocked, mutate allowed)")


def shell_and_trust():
    with tempfile.TemporaryDirectory(prefix="schedule-shell-") as temp:
        cwd = Path(temp)
        state = setup(cwd)
        pidfile = cwd / "child.pid"
        (cwd / "creds.env").write_text(f"GITHUB_TOKEN={SECRET}\n")
        # The command itself is persisted verbatim (it must re-run); output is redacted.
        command = f"cat {cwd / 'creds.env'}; sleep 30 & echo $! > {pidfile}; wait"
        write_store(state / "schedules.json", [job("poll", action="shell", command=command, tier="mutate",
                                                   wakeOn="never", timeoutMs=1500, prompt="")])
        project = dict(job("cloned", action="shell", command=f"touch {cwd / 'cloned-ran'}", tier="mutate", wakeOn="never", prompt=""))
        write_store(cwd / ".pig/schedule.json", [project])
        session = cwd / "session.jsonl"
        from .rpc import seed
        seed(session, cwd, [])
        with RPC(cwd, [EXT], session) as rpc:
            mark = len(rpc.events)
            rpc.call("new_session")
            row = wait_for(lambda: (r := read_store(state / "schedules.json")["poll"]).get("lastShell") and r, what="shell result")
            shell = row["lastShell"]
            assert shell["killed"] is True and shell["ok"] is False, shell
            assert SECRET not in json.dumps(row) and "GITHUB_TOKEN=[REDACTED]" in shell["stdout"], shell
            pid = int(pidfile.read_text())
            wait_for(lambda: not os.path.exists(f"/proc/{pid}") or open(f"/proc/{pid}/stat").read().split()[2] == "Z",
                     timeout=10, what="timed-out child process to be killed")
            held = wait_for(lambda: [n for n in notes(pump(rpc)[mark:]) if "held back 1 project job(s)" in n[1]], what="trust notice")
            assert "cloned" in held[0][1], held
            assert not (cwd / "cloned-ran").exists(), "untrusted project shell job ran"
            assert read_store(cwd / ".pig/schedule.json")["cloned"]["runCount"] == 0
        print("PASS schedule real host: shell timeout kills the process tree, persisted output redacted, untrusted project job held back")


def tool_and_skill():
    calls = []

    def script(messages):
        last = messages[-1]
        if last.get("role") == "tool":
            calls.append(text_of(last))
            return ("text", "ok")
        return ("tool", "schedule", dict(action="create", name="stretch", kind="notify", prompt="Stand up.", every="1h"))

    with tempfile.TemporaryDirectory(prefix="schedule-tool-") as temp:
        cwd = Path(temp)
        with ScriptedProvider(script) as provider:
            state = setup(cwd, provider)
            with RPC(cwd, [EXT], model=provider.model, skills=True) as rpc:
                _, rows = rpc.call("get_commands")
                commands = [c.get("name") for c in rows[0]["data"]["commands"]] if isinstance(rows[0].get("data"), dict) else []
                skill = state / "skills/schedule/SKILL.md"
                assert skill.exists() and skill.read_bytes() == (EXT / "skills/schedule/SKILL.md").read_bytes()
                assert "skill:schedule" in commands, commands
                rpc.call("prompt", message="Remind me hourly to stretch.")
                rpc.wait_event("agent_end")
            stored = list(read_store(state / "schedules.json").values())
            assert len(stored) == 1 and stored[0]["action"] == "notify" and stored[0]["tier"] == "read_only", stored
            assert calls and "Created job" in calls[0], calls
            system = provider.snapshot()[0]["messages"][0]
            assert "schedule" in text_of(system), "tool not offered to the model"
    print("PASS schedule real host: model-driven create via tool, skill materialized and discovered")


if __name__ == "__main__":
    privilege_and_ownership()
    queued_tiers()
    shell_and_trust()
    tool_and_skill()
