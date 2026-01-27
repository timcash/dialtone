# Branch: blender-ml-plugin
# Tags: p1, enhancement, 3d-graphics, machine-learning

# Goal
Integrate Blender's Python API and Machine Learning tools into a Dialtone plugin to enable programmatic 3D scene generation and analysis, inspired by the VIGA repository.

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start blender-ml-plugin`
- test-description: verify ticket is scaffolded and branch created
- test-command: `dialtone.sh test ticket blender-ml-plugin`
- status: done

## SUBTASK: Research VIGA and Blender API
- name: research-viga-blender
- description: Research the VIGA repository for ML inspiration and the Blender Python API for scene manipulation.
- test-description: Document key findings in the collaborative notes.
- test-command: `ls tickets/blender-ml-plugin/research.md`
- status: todo

## SUBTASK: Scaffold blender-ml plugin
- name: scaffold-plugin
- description: Create the `blender-ml` plugin structure in `src/plugins/blender-ml`.
- test-description: Verify the plugin builds and is recognized by Dialtone.
- test-command: `./dialtone.sh plugin build blender-ml`
- status: todo

## SUBTASK: Implement basic Blender scene generation
- name: blender-scene-gen
- description: Implement a basic command to generate a simple 3D scene using the Blender API.
- test-description: Verify a `.blend` file or rendered image is produced.
- test-command: `./dialtone.sh blender-ml generate --cube`
- status: todo

## SUBTASK: Integrate ML tools for scene analysis
- name: ml-scene-analysis
- description: Integrate ML models (inspired by VIGA) to analyze or augment 3D scenes.
- test-description: Verify ML models can process scene data.
- test-command: `./dialtone.sh blender-ml analyze --scene scene.blend`
- status: todo

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done blender-ml-plugin`
- status: todo

## Collaborative Notes
- **Reference**: https://github.com/timcash/dialtone/issues/89
- **VIGA Inspiration**: https://github.com/Fugtemypt123/VIGA
- Focus on `bpy` (Blender Python API) for scene control.

