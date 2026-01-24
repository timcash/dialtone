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

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done integrate-ffmpeg-camera-fix`
- status: done
