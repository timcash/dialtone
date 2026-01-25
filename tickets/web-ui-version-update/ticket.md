# Branch: web-ui-version-update
# Tags: ui, verification, deployment

# Goal
Verify the end-to-end build, deploy, and diagnostic loop by making a visible change to the Web UI version and confirming it on a remote robot.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start web-ui-version-update`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket web-ui-version-update`
- status: done

## SUBTASK: modify web ui text
- name: modify-ui-text
- description: update the initialization text in `src/web/src/main.ts` to include a version number (v1.0.1).
- test-description: verify the string is updated in the source.
- test-command: `grep "v1.0.1" src/web/src/main.ts`
- status: done

## SUBTASK: build and deploy UI
- name: build-deploy-ui
- description: run a full build and deploy the changes to the robot.
- test-description: verify build success and deployment.
- test-command: `./dialtone.sh build --full && ./dialtone.sh deploy`
- status: done

## SUBTASK: verify remote diagnostics and logs
- name: verify-remote
- description: run remote diagnostics and check logs to ensure the system is stable.
- test-description: verify diagnostics pass.
- test-command: `./dialtone.sh diagnostic && ./dialtone.sh logs --remote`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done web-ui-version-update`
- status: done

