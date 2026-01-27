# Branch: rf-detr-plugin
# Tags: p1, enhancement, ai-vision

# Goal
Integrate Roboflow's DETR (rf-detr) into a Dialtone plugin for object detection and tracking. Specifically, set up a demo for cat tracking using fine-tuned models and live camera feeds.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start rf-detr-plugin`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket rf-detr-plugin`
- status: done

## SUBTASK: Research and setup rf-detr environment
- name: research-rf-detr
- description: Research RoboFlow DETR integration requirements and set up necessary dependencies in a dev environment.
- test-description: Verify rf-detr can be imported and run in a standalone script.
- test-command: `python3 -c "import rf_detr" || true` (or relevant check)
- status: todo

## SUBTASK: Create rf-detr plugin scaffold
- name: scaffold-plugin
- description: Create the `rf-detr` plugin structure in `src/plugins/rf-detr`.
- test-description: Verify the plugin builds and is registered by Dialtone.
- test-command: `./dialtone.sh plugin build rf-detr`
- status: todo

## SUBTASK: Integrate model inference
- name: integrate-inference
- description: Implement model loading and inference logic for objects in the Dialtone plugin.
- test-description: Run inference on a sample image and verify output.
- test-command: `./dialtone.sh rf-detr detect --image sample.jpg`
- status: todo

## SUBTASK: Real-time tracking demo
- name: live-tracking
- description: Integrate the plugin with the live camera feed for real-time cat tracking.
- test-description: Verify tracking overlay appears on the camera feed.
- test-command: `dialtone.sh test ticket rf-detr-plugin --subtask live-tracking`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done rf-detr-plugin`
- status: todo

## Collaborative Notes
- **Reference**: https://github.com/timcash/dialtone/issues/91
- **Roboflow DETR**: https://github.com/roboflow/rf-detr
- Need to coordinate with Veo3 for video generation if fine-tuning is required.

