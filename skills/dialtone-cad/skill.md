# Dialtone CAD Skill
Dialtone CAD focuses on modeling, simulation, and digital twin workflows for robotic hardware and control systems. It supports design validation before hardware deployment.

## Core Focus
- Provide CAD and simulation tooling for hardware changes.
- Validate control logic in a digital twin environment.
- Support mesh transformations and physics simulations.

## Capabilities
- Assist with modeling and 3D printing preparation.
- Run FEA, CFD, thermal, and EMI simulations.
- Integrate localization and mapping algorithms for simulation.

## Inputs
- CAD models or geometry assets.
- Simulation parameters and constraints.
- Target hardware materials and tolerances.

## Outputs
- Simulation results and validation reports.
- Transformed meshes or model exports.
- Recommendations for hardware adjustments.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for CAD features.
- `docs/workflows/subtask_expand.md` for simulation steps.
- `docs/workflows/issue_review.md` for model regressions.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh plugin` for integration scaffolding.
- `./dialtone.sh logs` for simulation output capture.
- `./dialtone.sh deploy` for model distribution.

## Example Tasks
- Validate a new arm design with FEA.
- Create a digital twin for a control logic test.
- Convert a mesh to match a target topology.

## Notes
- Keep model versions and units explicit.
- Document simulation assumptions and limits.
