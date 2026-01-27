# Branch: gemini-cli-integration
# Tags: <tags>

# Goal
<goal>

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: to start work run the cli command `dialtone.sh ticket start gemini-cli-integration`
- test-description: run the ticket tests to verify that the ticket is in a valid state
- test-command: `dialtone.sh test ticket gemini-cli-integration`
- status: done

## SUBTASK: Create install and build files for AI plugin
- name: create-install-build
- description: Create src/plugins/ai/install.go and src/plugins/ai/build.go.
- test-command: `ls src/plugins/ai/install.go src/plugins/ai/build.go`
- status: done

## SUBTASK: Implement Gemini Command Logic
- name: implement-gemini-cmd
- description: Create src/plugins/ai/cli/gemini.go with RunGemini function and update src/plugins/ai/cli/ai.go to dispatch it.
- test-command: `./dialtone.sh ai --gemini "hello"`
- status: done

## SUBTASK: Implement optional geminicli installation
- name: install-geminicli
- description: Update src/plugins/ai/install.go to optionally install geminicli tool.
- test-command: `./dialtone.sh ai install geminicli`
- status: done

## SUBTASK: Implement geminicli proxy logic
- name: implement-proxy-logic
- description: Update src/plugins/ai/cli/gemini.go to proxy commands to geminicli and return results.
- test-command: `./dialtone.sh ai --gemini "what is dialtone?"`
- status: done

## SUBTASK: Implement auth key check and link
- name: implement-auth-check
- description: Detect missing GOOGLE_API_KEY and provide authentication link.
- test-command: `unset GOOGLE_API_KEY && ./dialtone.sh ai --gemini "test"`
- status: done

## SUBTASK: Update AI plugin README
- name: update-readme
- description: Update src/plugins/ai/README.md with --gemini details.
- test-command: `grep -- "--gemini" src/plugins/ai/README.md`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket, git is in the correct state and a pull request is created and ready for review. if it comepletes it should mark the final subtask as done
- test-description: vailidates all ticket subtasks are done
- test-command: `dialtone.sh ticket done gemini-cli-integration`
- status: done

