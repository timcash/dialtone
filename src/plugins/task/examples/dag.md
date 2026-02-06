# Task Dependency Graph

The following graph illustrates the 5-layer dependency structure for the "Secure API V2 Rollout".

## Legend
| Layer | Color | Description |
|---|---|---|
| **1. Foundation** | <span style="color:red">█</span> Red | Initial setup, config, and database migrations. |
| **2. Core Logic** | <span style="color:orange">█</span> Orange | Backend services, middleware, and core business logic. |
| **3. Features** | <span style="color:yellow">█</span> Yellow | User-facing APIs and documentation updates. |
| **4. QA** | <span style="color:blue">█</span> Blue | Testing, security scans, and performance verification. |
| **5. Release** | <span style="color:green">█</span> Green | Final deployment verification and sign-off. |

```mermaid
---
config:
  theme: dark
---
flowchart TD
    %% Global Nodes
    
    %% Layer 1: Foundation
    L1_1[env-config-update]
    L1_2[database-migration-users]
    
    %% Layer 2: Core Logic
    L2_1[auth-middleware-v2]
    L2_2[rate-limiter-impl]
    L2_3[audit-logger-service]
    
    %% Layer 3: Feature Implementation
    L3_1[user-profile-api]
    L3_2[admin-stats-api]
    L3_3[auth-docs-update]
    
    %% Layer 4: Quality Assurance
    L4_1[auth-tests-fix]
    L4_2[api-load-test]
    L4_3[security-scan-report]
    
    %% Layer 5: Release
    L5_1[auth-deployment-verify]

    %% Dependencies
    
    %% Layer 1 -> Layer 2
    L1_1 --> L2_1
    L1_1 --> L2_2
    L1_2 --> L2_1
    L1_2 --> L2_3
    
    %% Layer 2 -> Layer 3
    L2_1 --> L3_1
    L2_1 --> L3_3
    L2_3 --> L3_2
    
    %% Layer 2/3 -> Layer 4
    L2_1 --> L4_1
    L2_2 --> L4_2
    L3_1 --> L4_2
    L3_2 --> L4_2
    L3_1 --> L4_3
    
    %% Layer 4 -> Layer 5
    L4_1 --> L5_1
    L4_2 --> L5_1
    L4_3 --> L5_1
    L3_3 --> L5_1

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;
    classDef layer5 stroke:#00FF00,stroke-width:2px;

    class L1_1,L1_2 layer1;
    class L2_1,L2_2,L2_3 layer2;
    class L3_1,L3_2,L3_3 layer3;
    class L4_1,L4_2,L4_3 layer4;
    class L5_1 layer5;
```
