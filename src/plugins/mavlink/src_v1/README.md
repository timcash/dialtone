# MAVLink Plugin (`src_v1`)

```bash
# Generic workflow
./dialtone.sh mavlink src_v1 run --endpoint serial:/dev/ttyAMA0:57600 --nats-url nats://127.0.0.1:4222
./dialtone.sh mavlink src_v1 params --endpoint serial:/dev/ttyAMA0:57600
./dialtone.sh mavlink src_v1 key-params --endpoint serial:/dev/ttyAMA0:57600 --json
./dialtone.sh mavlink src_v1 test

# Run bridge (publishes MAVLink to NATS and consumes rover.command)
./dialtone.sh mavlink src_v1 run --endpoint serial:/dev/ttyAMA0:57600 --nats-url nats://127.0.0.1:4222

# Query key Rover params for RC mapping / tuning
./dialtone.sh mavlink src_v1 key-params --endpoint serial:/dev/ttyAMA0:57600 --json

# Remote stream + command smoke test over ssh mesh (no publish/UI required)
./dialtone.sh mavlink src_v1 stream --host rover --duration 5s --cmd stop
./dialtone.sh mavlink src_v1 stream --host rover --duration 5s --cmd mode --mode STEERING
```

## Purpose

`mavlink src_v1` bridges:
- MAVLink telemetry -> NATS subjects (`mavlink.*`)
- rover control commands (`rover.command`) -> MAVLink control messages

It is the control path used by Robot UI drive/guided buttons.

## RC Override Control Contract (Rover)

Current drive controls use `RC_CHANNELS_OVERRIDE` (MAVLink `common` dialect), with channel mapping learned from:
- `RCMAP_STEERING`
- `RCMAP_THROTTLE`

The service transmits override messages at:
- `20Hz` (`50ms` interval)

Important details for reliable Rover control:
- Send steering + throttle in the same override stream when both are desired.
- Keep streaming during motion; if messages stop, override can timeout/fall back.
- Use explicit stop (`stop`) to neutral + release both channels.
- For one-shot buttons, use a finite pulse duration and auto-stop afterward.

## Current Drive Commands

`rover.command` payload fields:
- `cmd` (required)
- `mode` (for `cmd=mode`)
- `throttlePwm` (1000-2000)
- `steeringPwm` (1000-2000)
- `durationMs` (200-5000)
- `noStop` (optional bool)
- `steeringOnly` (optional bool; used with `drive_left/right`)

Common commands:
- `mode` with `manual|steering|guided`
- `drive_up`
- `drive_down`
- `drive_left`
- `drive_right`
- `stop`
- `guided_forward_1m`
- `guided_square_5m`
- `guided_hold`

Notes:
- `drive_left/right` can run as steering-only pulses (`steeringOnly=true`) so forward/reverse throttle can remain independent.
- `stop` is the hard stop path and should clear all active motion.

## Telemetry Feedback For Control Confirmation

The plugin publishes control feedback as:
- subject: `mavlink.control_feedback`
- type: `CONTROL_FEEDBACK`

Fields:
- `source` (`RC_CHANNELS` or `SERVO_OUTPUT_RAW`)
- `steering_channel`
- `throttle_channel`
- `steering_raw`
- `throttle_raw`
- `timestamp`

Use this to verify the rover is actually receiving/producing expected control positions.

## Guided Mode: Practical Options

Current guided button actions are implemented as coarse RC-override approximations while forcing `GUIDED` mode.

### Better Guided Option A (recommended next)

Use `SET_POSITION_TARGET_LOCAL_NED` in a body-relative frame:
- `+1m X` for "forward 1m"
- sequence of 4 waypoints for a `5m` square

Pros:
- true position goals instead of PWM timing guesses
- easier to make deterministic across battery/load

### Better Guided Option B

Build MAVLink mission items dynamically:
- upload short mission (square)
- switch to `AUTO`
- execute then return to `HOLD`/`MANUAL`

Pros:
- reusable for complex paths
- richer status/ack path

### Better Guided Option C

Use velocity control (`SET_POSITION_TARGET_*` velocity mask):
- drive for fixed time + heading hold

Pros:
- simpler than full mission upload
- more stable than raw RC pulse scripts

## Operator Guidance

- For direct wheel/throttle control, prefer `MANUAL`.
- `STEERING`/`GUIDED` can apply controller behavior that feels different from raw RC.
- Always confirm feedback (`mavlink.control_feedback`) and keep a reliable stop action available.
