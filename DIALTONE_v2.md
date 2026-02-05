# DIALTONE v2 (Virtual Librarian)

Dialtone v2 centers on a task DAG shared over Hyperswarm. Tasks are the composable unit of work, streamed by topic, signed on completion, and negotiated collaboratively between `DIALTONE:`, `LLM:`, and `USER:`.

## Goals
- Replace ticket/subtask workflows with a task-first DAG.
- Enable async, peer-to-peer task streams by topic.
- Make task state auditable via signatures and logs.
- Keep the interface simple: a log stream with context.

## Core Concept: Task as the Central Object
Tasks are the unit of composition. They can be:
- produced by any peer (`USER:`, `LLM:`, `DIALTONE:`),
- routed on Hyperswarm topics,
- claimed by any peer,
- signed on completion,
- automatically completed when dependencies resolve.

## Task Model
Each task is a node in a DAG.

Required fields:
- `id`: stable task identifier
- `title`: short descriptive name
- `topic`: Hyperswarm topic name
- `dependencies`: list of task ids
- `budget`: numeric budget (time, cost, or combined)
- `score`: current priority or quality score
- `success_probability`: 0.0 to 1.0
- `signatures_required`: count or list of required signers
- `status`: `open` | `claimed` | `blocked` | `done`
- `tags`: optional labels (e.g. `needs-review`)

Rules:
- A task is `done` when all dependencies are `done` and required signatures are present.
- Any peer can propose a dependency or tag in the stream.
- Tasks can require multiple signatures to complete.

## Task Stream (Hyperswarm)
Tasks are broadcast on a topic as an async stream:
- producers: `DIALTONE:`, `LLM:`, `USER:`
- consumers: any peer subscribed to the topic
- updates: add task, add dependency, add tag, claim, sign

Example topic layout:
- `tasks/core` for active work
- `tasks/review` for review-only tasks
- `tasks/ops` for infrastructure and ops work

## CLI (new plugin name: `task`)
The new plugin name is `task`. Every completion is signed with:

`./dialtone.sh task --sign <task-id>`

Suggested CLI primitives:
- `./dialtone.sh task add <task-id> --title "..." --topic <topic>`
- `./dialtone.sh task dep add <task-id> <depends-on-id>`
- `./dialtone.sh task tag add <task-id> <tag>`
- `./dialtone.sh task claim <task-id>`
- `./dialtone.sh task --sign <task-id>`
- `./dialtone.sh task list --topic <topic>`
- `./dialtone.sh task graph --topic <topic>`

## Log Stream Format
Dialtone v2 uses a simple, structured log stream. Any peer can suggest new tasks, dependencies, or tags in-stream.

```xml
<turn>
  <user>add a task to review the new test-condition</user>
  <llm>
    <action>Creating task in tasks/review</action>
    <command>./dialtone.sh task add review-test-condition --title "Review test-condition" --topic tasks/review</command>
  </llm>
</turn>
<turn>
  <dialtone-response id="dt-3K4MT-109QX">
    <message-length>512 bytes</message-length>
    <mode>task</mode>
    <context>
      <item>task created on topic `tasks/review`</item>
      <item>signatures_required: 2</item>
      <item>status: open</item>
    </context>
    <next-commands>
      <command>./dialtone.sh task claim review-test-condition</command>
      <command>./dialtone.sh task --sign review-test-condition</command>
    </next-commands>
  </dialtone-response>
</turn>
```

## Task Examples
1. set the git branch
2. review a test-condition
3. triage a list of issues in a markdown file
4. send an email
5. image analysis for anomalies
6. find a part in a supply chain
7. improve UI for readability
8. change code

## Dependency Behavior
- Tasks can depend on many other tasks.
- Dependencies are first-class updates in the stream.
- When all dependent tasks are `done`, the parent is marked `done`.

## Signatures
Completion is a signature event:
- Supports multiple required signers.
- Enforces that completion is collaborative.
- Logged for audit and replay.

`./dialtone.sh task --sign <task-id>`

## UI Concept (Task DAG Explorer)
Imagine a UI that:
- renders the task DAG as a collapsible graph,
- shows topic filters and tags (`needs-review`, `blocked`),
- highlights tasks waiting on signatures,
- provides a side panel showing the log stream,
- allows drag-to-rewire dependencies with confirmation,
- includes a "budget heatmap" for nodes (color by budget or score).

## Notes
Dialtone v2 is a distributed, collaborative task graph. The log stream is the interface, the DAG is the source of truth, and signatures are the completion contract.