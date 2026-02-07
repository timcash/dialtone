# Swarm Plugin: Decentralized Architecture (v2)

This DAG illustrates the "Dual-Swarm" architecture required for `autokv_v2` and `autolog_v2`. It separates **Discovery/Authorization** from **Replication/Data**.

## Legend
| Layer | Color | Description |
|---|---|---|
| **1. Initialization** | <span style="color:red">█</span> Red | Local setup, key generation, and Autobase handling. |
| **2. Discovery** | <span style="color:orange">█</span> Orange | KeySwarm operations, handshake, and authorization logic. |
| **3. Replication** | <span style="color:yellow">█</span> Yellow | DataSwarm connection and Corestore replication streams. |
| **4. Operations** | <span style="color:blue">█</span> Blue | App-level reads/writes (Append, Get, Put) and linearization. |

## Architecture Flow

```mermaid
---
config:
  theme: dark
---
graph TD
    %% Global Nodes
    User[User / App Entry]

    %% Layer 1: Initialization
    L1_1["1. Initialize AutoBase"]
    L1_2["2. Generate Local Writer Key"]

    %% Layer 2: Discovery & Auth
    L2_1["3. Join 'Topic:Bootstrap'"]
    L2_2["4. Broadcast 'WRITER_KEY'"]
    L2_3["5. Receive Peer Keys"]
    L2_4["6. Autobase.addWriter(peerKey)"]

    %% Layer 3: Replication
    L3_1["7. Join 'Topic:Main'"]
    L3_2["8. On Connection"]
    L3_3["9. Corestore.replicate(stream)"]

    %% Layer 4: Data Operations
    L4_1["10. Append(Data)"]
    L4_2["11. Autobase.update() / Linearize"]
    L4_3["12. Hyperbee/Log View Updated"]

    %% Dependencies
    
    %% Layer 1
    User --> L1_1
    L1_1 --> L1_2

    %% Layer 1 -> Layer 2
    L1_2 --> L2_1
    L2_1 --> L2_2
    L2_1 --> L2_3
    L2_3 -- "Verify" --> L2_4

    %% Layer 2 -> Layer 3
    L2_4 -- "Authorized" --> L3_1
    L3_1 --> L3_2
    L3_2 --> L3_3

    %% Layer 3 -> Layer 4
    L3_3 --> L4_2
    
    %% App Interactions
    User -- "Write" --> L4_1
    L4_1 --> L4_2
    L4_2 --> L4_3
    L4_3 --> User

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;

    class L1_1,L1_2 layer1;
    class L2_1,L2_2,L2_3,L2_4 layer2;
    class L3_1,L3_2,L3_3,L4_1,L4_3 layer3;
    class L4_2 layer4;
```
