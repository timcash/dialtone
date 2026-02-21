# noisecan-sim2real
### signature:
- status: wait
- issue: 262
- source: github
- url: https://github.com/timcash/dialtone/issues/262
- synced-at: 2026-02-21T19:50:23Z
### sync:
- github-updated-at: 2026-02-14T21:05:57Z
- last-pulled-at: 2026-02-21T19:50:23Z
- last-pushed-at: 
- github-labels-hash: 
### description:
- create a "noiseCAN" framework for high-fidelity Sim-to-Real robotics
- simulate embedded environment noise: CAN packet latency, thread jitter, IMU integration drift
- implement Processor-in-the-Loop (PIL) by running real firmware in MuJoCo
- model LS6DSOX raw data streaming over an I2C emulator
- use xioTechnologies/Fusion library for real-time gravity projection in simulation
### tags:
- todo
- robotics
- sim2real
- canbus
- firmware
- mujoco
### comments-github:
- none
### comments-outbound:
- TODO: add a bullet comment here to post to GitHub
### task-dependencies:
- none
### documentation:
- https://github.com/xioTechnologies/Fusion
### test-condition-1:
- Firmware integration in MuJoCo maintains stability despite induced CAN latency of 10ms
### test-command:
- `./dialtone.sh robot test-sim2real`
### reviewed:
### tested:
### last-error-types:
- none
### last-error-times:
- none
### log-stream-command:
- TODO
### last-error-loglines:
- none
### notes:
- title: create noiseCAN for sim2real controls
- state: OPEN
- author: timcash
- created-at: 2026-02-14T21:05:57Z
- updated-at: 2026-02-14T21:05:57Z
