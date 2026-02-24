# REPL Multiplayer Plan

## Goal
- Support one DIALTONE server executing subtone commands for all users.
- Route all user input over NATS first.
- Use plugin-consistent room command: `/repl src_v1 join <room-name>`.
- Default room: `index`.

## Command Model
- Non-slash input publishes `chat` frames to the current room subject.
- Slash input publishes `command` frames to `repl.cmd`.
- DIALTONE server parses command frames.
- `/repl src_v1 join <room-name>` triggers room transition for that user.

## NATS Subjects
- `repl.cmd`: global command bus.
- `repl.room.<room>`: room event stream (`chat`, `line`, `join`, `left`, `server`, `heartbeat`, `control`, `error`).

## Room Transition
1. User in room A sends `/repl src_v1 join B`.
2. Server publishes `[LEFT]` in room A.
3. Server publishes targeted `control` frame instructing user to join room B.
4. Client switches subscription to room B and publishes `[JOIN]` in room B.

## Tests
- New suite: `src/plugins/repl/src_v1/test/99_multiplayer`.
- New command: `./dialtone.sh repl src_v1 test multiplayer`.
- Assertions:
  - three users join index room,
  - chat frames remain chat (not executed),
  - room switch command produces left/control/join sequence,
  - server executes a slash command via subtone and publishes line output,
  - all users emit left on quit.
