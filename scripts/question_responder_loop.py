#!/usr/bin/env python3
import time
import sys
from datetime import datetime, timezone
from pathlib import Path


def now_utc() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")


def extract_questions(content: str) -> list[str]:
    questions: list[str] = []
    for raw in content.splitlines():
        line = raw.strip()
        if not line:
            continue
        if line.startswith("#"):
            continue
        questions.append(line)
    return questions


def build_reply(questions: list[str]) -> str:
    out: list[str] = []
    out.append("# Replies")
    out.append(f"_Updated: {now_utc()}_")
    out.append("")
    if not questions:
        out.append("No questions found in `question.md`.")
        out.append("")
        return "\n".join(out)

    for idx, q in enumerate(questions, start=1):
        out.append(f"{idx}. Q: {q}")
        out.append(f"   A: Confirmed. I read this question at {now_utc()}.")
    out.append("")
    return "\n".join(out)


def main() -> int:
    question_file = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("question.md")
    reply_file = Path(sys.argv[2]) if len(sys.argv) > 2 else Path("reply.md")
    interval_seconds = 10

    print(f"[question-responder] watching: {question_file}")
    print(f"[question-responder] writing:  {reply_file}")
    print(f"[question-responder] interval: {interval_seconds}s")

    while True:
        if question_file.exists():
            content = question_file.read_text(encoding="utf-8")
            questions = extract_questions(content)
            reply_file.write_text(build_reply(questions), encoding="utf-8")
            print(f"[question-responder] wrote reply at {now_utc()}")
        time.sleep(interval_seconds)


if __name__ == "__main__":
    raise SystemExit(main())
