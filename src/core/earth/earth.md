This is a professional **Technical Specification & Development Ticket** for the procedural Three.js orbital simulation.

***

# Ticket: Procedural LEO Earth & ISS Simulation

## 1. Project Overview
**Objective:** Develop a purely procedural 3D simulation of Earth as seen from Low Earth Orbit (LEO). The project will use Three.js and custom GLSL shaders to generate planet surface features and atmospheric layers without the use of external texture files.

**Stack:**
*   **Bundler:** Vite
*   **Language:** TypeScript
*   **Library:** Three.js (Core & WebGL Shader Material)
*   **Asset Philosophy:** 100% Procedural (No PNG/JPG inputs).

---

## 2. Scene Architecture

### 2.1 Layered Sphere Construction
The planet will be composed of multiple concentric `SphereGeometry` layers:

| Layer Name | Hierarchy | Description |
| :--- | :--- | :--- |
| **Surface (The Globe)** | Inner | A single shader material handling height-based biomes: Ocean, Land, Mountains, and Rivers using Perlin or Simplex noise. |
| **Cloud Layer 1** | Middle | A slightly larger sphere (+0.05 radius) using low-frequency noise for large weather systems. |
| **Cloud Layer 2** | Outer | A sphere (+0.08 radius) using high-frequency noise for wispy, localized cloud detail. |
| **Atmospheric Glow** | Rim | A Fresnel-based shader to create the blue "limb" glow visible in the reference image. |

### 2.2 Procedural Shader Requirements (GLSL)
*   **Noise Algorithm:** Implement a simple 3D noise function (e.g., Classical Perlin) inside the fragment shaders.
*   **Surface Logic:** 
    *   `Value < 0.4`: **Ocean** (Deep blue with specular highlights).
    *   `Value 0.4 - 0.45`: **Rivers** (Narrow blue veins branching through land).
    *   `Value 0.45 - 0.7`: **Land** (Green to brown gradient).
    *   `Value > 0.7`: **Mountains** (Grey to white "snow cap" peaks).
*   **Cloud Logic:** Alpha-discard or transparency blending based on noise thresholds to create gaps in the clouds.

### 2.3 The LEO Satellite (ISS)
*   **Model:** A `Group` of primitive meshes (Box, Cylinder) representing a central habitat and blue solar arrays.
*   **Orbit:** A circular path at `Earth_Radius + Orbit_Height`. 
*   **Camera:** Fixed to the ISS group. The camera should point toward the Earth's curvature (The "Limb") to replicate the perspective in the provided reference photo.

---

## 3. Technical Requirements

### 3.1 Animation & Physics
*   **Orbital Velocity:** The ISS must orbit the globe continuously.
*   **Rotation:** The Earth and cloud layers should rotate at independent speeds to simulate atmospheric drift.

### 3.2 Automated Test Suite
Upon initialization, the script must execute a `TestRunner` class that logs the following to the browser console:
1.  **Geometry Verification:** Confirm vertex counts and radii for all 3 layers.
2.  **Shader Compilation:** Verify that the custom GLSL materials are linked correctly.
3.  **Orbital Stability:** Log the distance between the camera (ISS) and the Earth center to ensure the orbit isn't decaying or drifting.
4.  **Color Verification:** Log the hex codes used for the biomes (Ocean, Land, Mountain) for visual audit.

---

## 4. Visual Reference Goals
*   **Viewpoint:** High-angle horizon shot (The Earth occupies the bottom 60% of the frame).
*   **Lighting:** Strong directional light (The Sun) causing high-contrast shadows on the clouds and specular glints on the oceans.
*   **Background:** Solid black (`#000000`) to represent the vacuum of space.

---

**Next Step:** Once this ticket is approved, I will provide the full `globe.ts` and shader implementation. Would you like me to proceed with the code?