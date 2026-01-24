# Branch: integrate-ffmpeg-camera-fix
# Tags: camera, ffmpeg, linux

# Goal
Resolve camera stability issues on Raspberry Pi by implementing robust frame buffering and YUYV fallback in the native Go implementation, avoiding external ffmpeg dependencies.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start integrate-ffmpeg-camera-fix`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh ticket test integrate-ffmpeg-camera-fix`
- status: done

## SUBTASK: Implement Frame Buffer Pattern
- name: implement-frame-buffer
- description: Modify `src/camera_linux.go` to use a background goroutine that captures frames into a `sync.RWMutex` protected buffer. Handlers should poll this buffer instead of reading directly from the cam channel.
- test-description: Run `dialtone.sh build` and test streaming to multiple browser tabs.
- test-command: `./dialtone.sh test`
- status: done

## SUBTASK: Implement YUYV Fallback
- name: implement-yuyv-fallback
- description: Add logic to `StartCamera` to try MJPEG first. If it fails, retry with YUYV format utilizing software JPEG encoding.
- test-description: Set `DIALTONE_CAMERA_FORMAT=yuyv` and verify stream works.
- test-command: `./dialtone.sh test`
- status: done

## SUBTASK: Answer User Question
- name: answer-user-embed-question
- description: Explain the trade-offs of embedding ffmpeg (size, cross-compilation) vs fixing the Go code.
- test-description: Verify explanation in chat context.
- test-command: N/A
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done integrate-ffmpeg-camera-fix`
- status: todo
