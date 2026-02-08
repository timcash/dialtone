# Smoke Test Orchestration DAG

This document visualizes the `www` plugin smoke test architecture, detailing the flow from Go-based orchestration to Browser-based verification.

## Legend
| Layer | Color | Description |
|---|---|---|
| **1. Foundation** | <span style="color:red">█</span> Red | Initialization of the test environment (directories, cleanups). |
| **2. Core Logic** | <span style="color:orange">█</span> Orange | Preparing the runtime (Dev Server, Browser, Websocket connection). |
| **3. Features** | <span style="color:yellow">█</span> Yellow | The active test loop controlling the browser navigation. |
| **4. QA** | <span style="color:blue">█</span> Blue | Data capture, verification, and visual evidence collection. |
| **5. Release** | <span style="color:green">█</span> Green | Processing results into human-readable reports and final status. |

```mermaid
---
config:
  theme: dark
---
flowchart TD
    %% Global Nodes
    
    %% Layer 1: Foundation
    L1_1[env-check-dirs]
    L1_2[clean-screenshots]
    
    %% Layer 2: Core Logic
    L2_1{check-port-5173}
    L2_2[start-dev-server]
    L2_3[connect-chrome]
    L2_4[enable-cdp-perf]
    L2_5[inject-observers]
    
    %% Layer 3: Feature Implementation
    L3_1[nav-base-get-sections]
    L3_2[trigger-proof-of-life]
    L3_3[nav-section-hash]
    L3_4[wait-stable-stats]
    
    %% Layer 4: Quality Assurance
    L4_1[capture-metrics]
    L4_2[viewport-screenshot]
    L4_3[verify-hash-scroll]
    L4_4[log-console-errors]
    
    %% Layer 5: Release
    L5_1[filter-logs]
    L5_2[generate-smoke-md]
    L5_3[tile-summary-png]
    L5_4{final-status-check}

    %% Dependencies
    
    %% Layer 1 -> Layer 2
    L1_1 --> L1_2
    L1_2 --> L2_1
    
    L2_1 -- No --> L2_2
    L2_2 --> L2_3
    L2_1 -- Yes --> L2_3
    
    L2_3 --> L2_4
    L2_4 --> L2_5
    
    %% Layer 2 -> Layer 3
    L2_5 --> L3_1
    L3_1 --> L3_2
    L3_2 --> L3_3
    
    %% Layer 3 -> Layer 4 (The Loop)
    L3_3 --> L3_4
    L3_4 --> L4_1
    L4_1 --> L4_2
    L4_2 --> L4_3
    L4_3 --> L4_4
    
    L4_4 -->|Next Section| L3_3
    
    %% Layer 4 -> Layer 5
    L4_4 -->|Loop Done| L5_1
    L5_1 --> L5_2
    L5_2 --> L5_3
    L5_3 --> L5_4

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;
    classDef layer5 stroke:#00FF00,stroke-width:2px;

    class L1_1,L1_2 layer1;
    class L2_1,L2_2,L2_3,L2_4,L2_5 layer2;
    class L3_1,L3_2,L3_3,L3_4 layer3;
    class L4_1,L4_2,L4_3,L4_4 layer4;
    class L5_1,L5_2,L5_3,L5_4 layer5;
```
