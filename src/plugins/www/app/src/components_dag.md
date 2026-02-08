# WWW Component Architecture DAG

This document visualizes the standard architecture for `www` app components, detailing the lifecycle from the main router down to individual visualizations and menu integration.

## Legend
| Layer | Color | Description |
|---|---|---|
| **1. Foundation** | <span style="color:red">█</span> Red | The Application Entry and Section Manager (Router/Loader). |
| **2. Core Logic** | <span style="color:orange">█</span> Orange | The Component Factory (`mountX`) and State/Config definitions. |
| **3. Features** | <span style="color:yellow">█</span> Yellow | The Concrete Implementation (Three.js Viz, DOM Overlay). |
| **4. QA/Interact** | <span style="color:blue">█</span> Blue | User Interactivity via the Global Menu System. |
| **5. Release** | <span style="color:green">█</span> Green | Lifecycle Management (Visibility, Disposal, Cleanup). |

```mermaid
---
config:
  theme: dark
---
flowchart TD
    %% Global Nodes
    
    %% Layer 1: Foundation (Router)
    L1_1[Main Entry: main.ts]
    L1_2[SectionManager]
    L1_3((Lazy Load: import))
    
    %% Layer 2: Core Logic (Factory)
    L2_1[Mount Function: mountAbout]
    L2_2[State: Config Objects]
    L2_3[Container: InnerHTML]
    
    %% Layer 3: Features (Impl)
    L3_1[Class: VisionVisualization]
    L3_2[DOM: Marketing Overlay]
    L3_3[ThreeJS: Scene/Renderer]
    
    %% Layer 4: QA/Interact (Menu)
    L4_1[Event: setVisible=true]
    L4_2[Menu.clear]
    L4_3[Menu.addSlider/Header]
    L4_4{User Input}
    
    %% Layer 5: Release (Lifecycle)
    L5_1[Update: Apply Config]
    L5_2[Event: setVisible=false]
    L5_3[Cleanup: dispose]

    %% Dependencies
    
    %% Layer 1 -> Layer 2
    L1_1 --> L1_2
    L1_2 -->|Intersection| L1_3
    L1_3 --> L2_1
    
    %% Layer 2 -> Layer 3
    L2_1 --> L2_2
    L2_1 --> L2_3
    L2_1 --> L3_1
    L2_3 --> L3_2
    L3_1 --> L3_3
    
    %% Layer 2 -> Layer 4 (Activation)
    L2_1 -->|Return API| L4_1
    L4_1 --> L4_2
    L4_2 --> L4_3
    
    %% Layer 4 -> Layer 5 (Interaction Loop)
    L4_4 --> L4_3
    L4_3 -->|Callback| L5_1
    L5_1 -->|Update| L2_2
    L5_1 -->|Update| L3_1
    
    %% Lifecycle
    L1_2 -->|Scroll Away| L5_2
    L5_2 -->|Cleanup Menu| L4_2
    L5_2 -->|Pause| L3_1
    L1_2 -->|Unload| L5_3
    L5_3 --> L3_1

    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;
    classDef layer5 stroke:#00FF00,stroke-width:2px;

    class L1_1,L1_2,L1_3 layer1;
    class L2_1,L2_2,L2_3 layer2;
    class L3_1,L3_2,L3_3 layer3;
    class L4_1,L4_2,L4_3,L4_4 layer4;
    class L5_1,L5_2,L5_3 layer5;
```

## Component Pattern Implementation

### 1. The Mount Function (Factory)
Every component exports a `mountX(container)` function. This acts as a closure that holds the component's **Configuration State** (e.g., `lightConfig`, `motionConfig`). This state is shared between the Visualization and the Menu.

### 2. The Visualization Class
The heavy lifting (Three.js, WebGL) happens in a dedicated class (e.g., `VisionVisualization`). It is initialized by the Mount function and exposes public methods (setters) to modify its behavior in real-time.

### 3. The Menu Integration
The menu is **rebuilt** every time the section becomes visible (`setVisible(true)`).
- `Menu.getInstance().clear()` is called to remove controls from the previous section.
- Sliders and inputs are added that directly modify the **Shared Config State** and call setters on the **Visualization Class**.
- When the section becomes hidden (`setVisible(false)`), the menu cleanup function is called (typically just clearing the menu again or stopping intervals).

### 4. Lifecycle API
The `mountX` function returns a standardized API object expected by `SectionManager`:
```typescript
{
    dispose: () => void;      // Full teardown (remove DOM, dispose WebGL)
    setVisible: (v: boolean) => void; // Toggle rendering/animation loop & Rebuild/Clear Menu
}
```
