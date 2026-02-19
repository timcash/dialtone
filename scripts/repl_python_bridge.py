#!/usr/bin/env python3
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
RUNTIME_DIR = ROOT / ".dialtone" / "run"
EVENTS_FILE = RUNTIME_DIR / "repl-events.log"
ACTIVE_SESSION_FILE = RUNTIME_DIR / "repl-active.session"
LOCK_FILE = RUNTIME_DIR / "repl-python-bridge.pid"
BRIDGE_BIN = os.environ.get("DIALTONE_LLM_OPS_BIN", str(ROOT / "scripts" / "llm_ops_demo_bridge.sh"))


def utc_ts() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def ensure_single_instance() -> None:
    RUNTIME_DIR.mkdir(parents=True, exist_ok=True)
    if LOCK_FILE.exists():
        old_pid = LOCK_FILE.read_text(encoding="utf-8").strip()
        if old_pid.isdigit():
            try:
                os.kill(int(old_pid), 0)
                print(f"[repl-python-bridge] already running with pid {old_pid}")
                sys.exit(0)
            except OSError:
                pass
    LOCK_FILE.write_text(str(os.getpid()), encoding="utf-8")


def read_active_session() -> str:
    if not ACTIVE_SESSION_FILE.exists():
        return ""
    return ACTIVE_SESSION_FILE.read_text(encoding="utf-8").strip()


def emit_llm_ops(line: str) -> None:
    payload = f"{utc_ts()}\tLLMOPS-PY\tOUTPUT\tLLM-OPS\tLLM-OPS> {line}\n"
    with EVENTS_FILE.open("a", encoding="utf-8") as f:
        f.write(payload)


def reply(question: str) -> list[str]:
    question = question.strip()
    if not question:
        return []
    try:
        out = subprocess.check_output([BRIDGE_BIN, question], stderr=subprocess.STDOUT, text=True)
    except subprocess.CalledProcessError as exc:
        out = exc.output or f"Bridge error (exit {exc.returncode})."
    except FileNotFoundError:
        out = "Bridge error: llm bridge executable not found."
    lines = [line.strip() for line in out.splitlines() if line.strip()]
    return lines or ["I am here."]


def main() -> None:
    ensure_single_instance()
    EVENTS_FILE.parent.mkdir(parents=True, exist_ok=True)
    EVENTS_FILE.touch(exist_ok=True)
    print(f"[repl-python-bridge] watching {EVENTS_FILE}")

    with EVENTS_FILE.open("r", encoding="utf-8") as f:
        f.seek(0, os.SEEK_END)
        while True:
            line = f.readline()
            if not line:
                time.sleep(0.15)
                continue
            parts = line.rstrip("\n").split("\t", 4)
            if len(parts) != 5:
                continue
            _ts, sid, kind, _role, text = parts
            if kind != "INPUT":
                continue
            if not text.startswith("@LLM-OPS "):
                continue
            active = read_active_session()
            if not active or sid != active:
                continue
            question = text[len("@LLM-OPS "):].strip()
            for out_line in reply(question):
                emit_llm_ops(out_line)


if __name__ == "__main__":
    try:
        main()
    finally:
        try:
            if LOCK_FILE.exists() and LOCK_FILE.read_text(encoding="utf-8").strip() == str(os.getpid()):
                LOCK_FILE.unlink()
        except OSError:
            pass
