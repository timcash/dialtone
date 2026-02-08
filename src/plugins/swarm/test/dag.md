# Swarm Plugin: Test Lifecycle DAG

This DAG illustrates the testing flow for `autokv` and `autolog` in a multi-process environment. It focuses on the **Sub-Process Test Runner** strategy to ensure independent verification.

## Legend
| Layer | Color | Description |
|---|---|---|
| **1. Orchestrator** | <span style="color:red">█</span> Red | The main test runner spawning sub-processes. |
| **2. Initialization** | <span style="color:orange">█</span> Orange | Sub-process startup, config, and topic derivation. |
| **3. Discovery Loop** | <span style="color:yellow">█</span> Yellow | The "Warm Topic" wait loop and peer finding logic. |
| **4. Interaction** | <span style="color:blue">█</span> Blue | Random reads/writes and state verification. |
| **5. Cleanup** | <span style="color:green">█</span> Green | Process termination and artifact removal. |

## Test Flow

```mermaid
---
config:
  theme: dark
---
graph TD
    %% Global Nodes
    Runner["Test Runner (Main Process)"]

    %% Layer 1: Orchestrator
    L1_1["1. Spawn Node A (Sub-process)"]
    L1_2["2. Spawn Node B (Sub-process)"]
    L1_3["3. Spawn Node C (Sub-process)"]

    %% Layer 2: Initialization (Per Node)
    L2_1["4. Init AutoBase & Corestore"]
    L2_2["5. Join 'Warm Topic'"]

    %% Layer 3: Discovery Loop
    L3_1{"6. Is Peer Found?"}
    L3_2["7. Wait / Retry (1s)"]
    L3_3["8. Exchange Keys (KeySwarm)"]
    L3_4["9. Authorize Writers"]

    %% Layer 4: Interaction
    L4_1{"10. Loop: Random Action"}
    L4_2["11. Append Log / Put KV"]
    L4_3["12. Verify State (Checksum)"]
    L4_4["13. Wait for Convergence"]

    %% Layer 5: Cleanup
    L5_1["14. Validation Success"]
    L5_2["15. Kill Sub-processes"]

    %% Dependencies
    
    %% Layer 1
    Runner --> L1_1
    Runner --> L1_2
    Runner --> L1_3

    %% Layer 1 -> Layer 2
    L1_1 --> L2_1
    L1_2 --> L2_1
    L1_3 --> L2_1
    L2_1 --> L2_2

    %% Layer 2 -> Layer 3
    L2_2 --> L3_1
    L3_1 -- "No" --> L3_2
    L3_2 --> L3_1
    L3_1 -- "Yes" --> L3_3
    L3_3 --> L3_4

    %% Layer 3 -> Layer 4
    L3_4 --> L4_1
    L4_1 --> L4_2
    L4_2 --> L4_3
    L4_3 -- "Mismatch" --> L4_4
    L4_4 --> L4_3
    L4_3 -- "Match" --> L5_1

    %% Layer 4 -> Layer 5
    L5_1 --> L5_2

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;
    classDef layer5 stroke:#00FF00,stroke-width:2px;

    class Runner,L1_1,L1_2,L1_3 layer1;
    class L2_1,L2_2 layer2;
    class L3_1,L3_2,L3_3,L3_4 layer3;
    class L4_1,L4_2,L4_3,L4_4 layer4;
    class L5_1,L5_2 layer5;
```
