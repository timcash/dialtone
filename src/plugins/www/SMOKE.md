# WWW Plugin Smoke Test Report

**Generated at:** Thu, 12 Feb 2026 10:59:39 PST

## 1. Expected Errors (Proof of Life)

| Level | Message | Status |
|---|---|---|
| error | "[PROOFOFLIFE] Intentional Browser Test Error" | ✅ CAPTURED |
| error | [PROOFOFLIFE] Intentional Go Test Error | ✅ CAPTURED |

## 2. Real Errors & Warnings

### [warning] 
```
"[main] ❌ loadSection failed: # not found or not a slide"

Stack Trace:
  loadSection (http://127.0.0.1:5173/src/main.ts:148:10)
   (http://127.0.0.1:5173/src/main.ts:151:5)

```

## 3. Performance Metrics

| Section | FPS | App CPU (ms) | App GPU (ms) | JS Heap (MB) | Resources (MB) | Status |
|---|---|---|---|---|---|---|
| s-home | 60 | 0.37 | 0.12 | 0.00 | 0.00 | OK |
| s-about | 62 | 1.11 | 3.15 | 0.00 | 0.00 | OK |
| s-robot | 60 | 0.70 | 0.02 | 0.00 | 0.00 | OK |
| s-neural | 63 | 1.09 | 0.02 | 0.00 | 0.00 | OK |
| s-math | 60 | 0.82 | 0.02 | 0.00 | 0.00 | OK |
| s-cad | 62 | 0.26 | 0.06 | 0.00 | 0.00 | OK |
| s-radio | 35 | 0.65 | 0.07 | 0.00 | 0.00 | OK |
| s-geotools | 61 | 0.28 | 0.08 | 0.00 | 0.00 | OK |
| s-docs | 61 | 0.22 | 0.01 | 0.00 | 0.00 | OK |
| s-policy | 61 | 0.58 | 0.17 | 0.00 | 0.00 | OK |
| s-music | 2 | 0.65 | 0.28 | 0.00 | 0.00 | OK |
| s-webgpu-template | 60 | 0.22 | 0.15 | 0.00 | 0.00 | OK |
| s-threejs-template | 60 | 0.21 | 0.01 | 0.00 | 0.00 | OK |

## 4. Test Orchestration DAG

### Legend
| Layer | Color | Description |
|---|---|---|
| **1. Foundation** | <span style="color:red">█</span> Red | Cleanup, environment, and directory setup. |
| **2. Core Logic** | <span style="color:orange">█</span> Orange | Dev server, browser initialization, and proof-of-life. |
| **3. Features** | <span style="color:yellow">█</span> Yellow | Navigation loop, verification, and metrics capture. |
| **4. QA** | <span style="color:blue">█</span> Blue | Screenshot capture and visual summary tiling. |
| **5. Release** | <span style="color:green">█</span> Green | Final report generation and process cleanup. |

```mermaid
graph TD
    %% Layer 1: Foundation
    L1[Setup: Cleanup & Dirs]
    
    %% Layer 2: Core Logic
    L2[Dev Server: npm run dev]
    L3[Browser: headless chrome]
    L0[Proof of Life: Deliberate error discovery]
    
    %% Layer 3: Feature Implementation
    L4[Navigation: Hash-based loop]
    L5[Verify: Hash & scroll position]
    L6[Metrics: CDP Performance Data]
    
    %% Layer 4: Quality Assurance
    L7[Screenshots: Capture per-section]
    L8[Tiling: summary.png]
    
    %% Layer 5: Release
    L9[Report: SMOKE.md]
    L10[Cleanup: Stop browser & dev server]
    %% Dependencies
    L1 --> L2
    L2 --> L3
    L3 --> L0
    L3 --> L4
    L4 --> L5
    L4 --> L6
    L4 --> L7
    L7 --> L8
    L0 --> L9
    L4 --> L9
    L6 --> L9
    L8 --> L9
    L9 --> L10
    %% Styling
    classDef layer1 stroke:#FF0000,stroke-width:2px;
    classDef layer2 stroke:#FFA500,stroke-width:2px;
    classDef layer3 stroke:#FFFF00,stroke-width:2px;
    classDef layer4 stroke:#0000FF,stroke-width:2px;
    classDef layer5 stroke:#00FF00,stroke-width:2px;
    
    class L1 layer1;
    class L2,L3,L0 layer2;
    class L4,L5,L6 layer3;
    class L7,L8 layer4;
    class L9,L10 layer5;
```

## 5. Visual Summary Grid

![Summary Grid](screenshots/summary.png)