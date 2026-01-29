# Name: earth-www-slide
# Tags: www, earth, ui, scroll

# Goal
Integrate the Earth V1 demo into the www site as the first slide section, remove the old globe.gl implementation, and preserve scroll-to-next-section behavior.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- tags: setup
- dependencies:
- description: run the cli command `dialtone.sh ticket start earth-www-slide`
- test-condition-1: verify ticket is scaffolded under src/tickets/earth-www-slide
- test-condition-2: ticket is set as current for follow-up commands
- agent-notes: keep this ticket focused on the www slide integration
- pass-timestamp: 2026-01-28T18:44:33-08:00
- fail-timestamp:
- status: done

## SUBTASK: review earth v1 guide and current www slide layout
- name: review-earth-v1
- tags: planning, docs
- dependencies: ticket-start
- description: read `src/core/earth/earth_v1.md` and identify the current www first section and globe.gl usage to map integration points
- test-condition-1: earth_v1.md notes summarized in agent-notes
- test-condition-2: location of existing globe.gl section is identified
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: integrate Earth V1 into first www section
- name: integrate-earth-section
- tags: www, ui
- dependencies: review-earth-v1
- description: use `./dialtone.sh www dev` to integrate the Earth demo into the first `<section>` of the www site
- test-condition-1: first `<section>` renders the Earth V1 scene when running `./dialtone.sh www dev`
- test-condition-2: Earth scene loads without console errors in the dev server
- agent-notes: keep section markup consistent with existing slide styles
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: remove legacy globe.gl implementation
- name: remove-globe-gl
- tags: cleanup
- dependencies: integrate-earth-section
- description: remove globe.gl code and any related assets no longer used by the www site
- test-condition-1: no remaining references to globe.gl in www source
- test-condition-2: build/dev server runs without globe.gl dependencies
- agent-notes: confirm unused assets are removed from www bundle
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: verify scroll snap still works between sections
- name: verify-scroll-snap
- tags: test, ui
- dependencies: remove-globe-gl
- description: confirm the Earth slide behaves like a scrollable slide and the next section still snaps correctly
- test-condition-1: scroll from first section to next section still snaps in `./dialtone.sh www dev`
- test-condition-2: no layout regressions or overlapping sections
- agent-notes: use existing scroll snap test or manual scroll check
- pass-timestamp:
- fail-timestamp:
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- tags: cli
- dependencies: verify-scroll-snap
- description: run the ticket cli to verify all steps to complete the ticket
- test-condition-1: validates all ticket subtasks are done
- test-condition-2: `./dialtone.sh ticket done earth-www-slide`
- agent-notes:
- pass-timestamp:
- fail-timestamp:
- status: todo
