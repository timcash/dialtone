# Ticket: Integrate FFMPEG/MJPEG-Streamer or Fix Camera Stability

## Status
- [ ] Draft
- [x] In Progress
- [ ] Done

## Goal
Resolve camera stability issues on Raspberry Pi by either integrating robust external tools (ffmpeg/mjpg-streamer) or upgrading the native Go implementation to production standards (YUYV fallback, buffering).

## Context
The current `src/camera_linux.go` implementation uses `go4vl` efficiently but lacks the robustness features described in the technical manual (YUYV fallback, frame buffering). The user is experiencing issues and asking about embedding `ffmpeg`.

## Plan
1.  Attempt to implement the robust "Latest Frame Buffer Pattern" and "YUYV Fallback" in pure Go first, as this maintains the single-binary architecture.
2.  If that is insufficient, investigate embedding or shipping `ffmpeg`.

## Subtasks

### SUBTASK: Implement Frame Buffer Pattern
- name: implement-frame-buffer
- description: Modify `src/camera_linux.go` to use a background goroutine that captures frames into a `sync.RWMutex` protected buffer. Handlers should poll this buffer instead of reading directly from the cam channel (which causes issues with multiple viewers).
- test-description: Run `dialtone.sh build` and test streaming to multiple browser tabs.
- test-command: `./dialtone.sh test` (requires manual verification for camera typically, or mocked test).
- status: todo

### SUBTASK: Implement YUYV Fallback
- name: implement-yuyv-fallback
- description: Add logic to `StartCamera` to try MJPEG first. If it fails (or produces 0 byte frames), close and retry with YUYV format, using `image/jpeg` to encode frames in software.
- test-description: Set `DIALTONE_CAMERA_FORMAT=yuyv` and verify stream works.
- test-command: `./dialtone.sh test`
- status: todo

### SUBTASK: Answer User Question
- name: answer-user-embed-question
- description: Explain the trade-offs of embedding ffmpeg (size, cross-compilation) vs fixing the Go code.
- test-description: N/A
- test-command: N/A
- status: todo
