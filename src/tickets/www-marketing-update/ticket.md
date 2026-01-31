# Branch: www-marketing-update
# Tags: www, marketing, threejs

# Goal
Update the marketing sections to tell a "now is the time to learn and build" story, center the Earth visualization, and move the buy button into the hero.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: start work by running `./dialtone.sh ticket start www-marketing-update`
- test-condition-1: run `./dialtone.sh plugin test <plugin-name>` to verify the ticket is valid
- test-condition-2: `./dialtone.sh plugin test <plugin-name>`
- tags:
- dependencies:
- agent-notes: ran `./dialtone.sh ticket start www-marketing-update`
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: create feature branch
- name: create-branch
- description: create the `www-marketing-update` branch before code changes
- test-condition-1: `git branch --show-current` returns `www-marketing-update`
- test-condition-2: `git status -sb` shows branch name
- tags:
- dependencies:
- agent-notes: created and switched to `www-marketing-update`
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: simplify Earth camera and center view
- name: earth-camera-center
- description: simplify the camera in `earth.ts` and keep the Earth centered in the viewport
- test-condition-1: verify the Earth stays centered while animating
- test-condition-2: `./dialtone.sh www dev` renders the Earth centered
- tags:
- dependencies:
- agent-notes: removed ISS/camera orbit and set fixed camera facing Earth
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: update marketing narrative
- name: marketing-story
- description: update all section copy to tell a "now is the time to learn and build" story about robotics, math, and networks
- test-condition-1: visual check of marketing text on each section
- test-condition-2: `./dialtone.sh www dev` shows updated copy
- tags:
- dependencies:
- agent-notes: rewrote copy for all sections to align with narrative
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: center and fade-in marketing text
- name: marketing-style
- description: make marketing text large, white, centered, and fade in on section entry
- test-condition-1: scroll sections and verify fade-in behavior
- test-condition-2: `./dialtone.sh www dev` shows centered overlay styling
- tags:
- dependencies:
- agent-notes: centered overlay styling and intersection-based fade-in
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: move buy button to hero
- name: move-buy-button
- description: place the Stripe buy button below the first section text and remove other Stripe text
- test-condition-1: verify only the button appears under the hero text
- test-condition-2: button opens Stripe link
- tags:
- dependencies:
- agent-notes: added hero CTA button and removed Stripe section
- pass-timestamp: 2026-01-31
- fail-timestamp:
- status: done

## SUBTASK: publish www site
- name: www-publish
- description: run `./dialtone.sh www publish` and verify the main website
- test-condition-1: publish completes without errors
- test-condition-2: main site reflects updates
- tags:
- dependencies:
- agent-notes: publish failed due to missing Vercel project settings; run `vercel pull --yes` then retry
- pass-timestamp:
- fail-timestamp: 2026-01-31
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: `./dialtone.sh ticket done www-marketing-update`
- tags:
- dependencies:
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo
