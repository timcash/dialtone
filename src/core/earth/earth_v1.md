 # Earth Project V1 Summary
 
 This document summarizes the current Earth orbital demo implementation and its major parts.
 
 ## Project Overview
 - A Three.js procedural Earth/LEO scene rendered with Vite and TypeScript.
 - All Earth, cloud, and atmosphere visuals are shader-driven (no external textures).
 - A UI “gizmo” panel provides live control of camera, animation, and lighting.
 
 ## Scene Composition
 - **Earth surface** (`earth`): Sphere using a custom GLSL shader for ocean/land/mountains/rivers.
 - **Cloud layer 1** (`cloud1`): Larger sphere with procedural noise and alpha cutouts.
 - **Cloud layer 2** (`cloud2`): Outer sphere with higher-frequency cloud noise.
 - **Atmospheric limb** (`atmosphere`): Fresnel glow shell with additive blending.
 - **Sun-scatter atmosphere** (`sunAtmosphere`): Extra shell for sun-facing rim glow.
 - **ISS group** (`issGroup`): Simple satellite made from primitive meshes.
 - **Sun glow mesh** (`sunGlow`): Visible sun disc (MeshBasicMaterial).
 
 ## Lighting Model
 - **Sun (PointLight)** (`sunLight`): Point source used as “sun” intensity.
 - **Key (DirectionalLight)** (`sunKeyLight`): Directional light for main shading direction.
 - **Ambient (AmbientLight)** (`ambientLight`): Low global fill.
 - The sun and key lights share the same orbit position each frame.
 - Light direction used in shaders is derived from the key light position.
 
 ## Shader System
 - **Noise**: Inlined GLSL simplex-style noise for terrain and clouds.
 - **Earth shader**:
   - Biome selection by noise value.
   - Light = ambient + key + sun term with boosted diffuse when ambient is low.
 - **Cloud shaders**:
   - Noise-based alpha for cloud breakup.
   - Lighting includes separate sun term for stronger highlights.
 - **Atmosphere shader**:
   - Fresnel rim term + lighting for limb glow.
 - **Sun-scatter atmosphere shader**:
   - View-dependent rim and sun-facing boost.
   - Uses camera position uniform to shape the rim.
 
 ## Camera & Orbit
 - Camera is attached to the ISS group each frame.
 - Camera uses a local offset plus Euler adjustments for fine control.
 - ISS orbits Earth on a simple parametric path.
 
 ## UI / Gizmo Controls
 - **Camera**: Pitch/Yaw/Roll sliders (degrees).
 - **Camera offset**: X/Y/Z sliders.
 - **Animation**:
   - Time scale (exponential 0–100).
   - Shader time scale.
   - Earth rotation speed.
   - Cloud rotation speeds.
   - ISS orbit speed.
 - **Lights**:
   - Key intensity.
   - Sun intensity (exponential 0–100).
   - Ambient intensity.
   - Shared light orbit height.
   - Sun orbit angle (degrees).
 - **Material**: Color scale.
 - **Timing stats**: FPS, dtRaw, dt.
 - **Copy JSON**: Outputs a snapshot of the current settings.
 
 ## Runtime Updates (per frame)
 - Updates shader time for clouds.
 - Updates ISS orbit position.
 - Positions sun and key lights on the shared orbit.
 - Updates shader uniforms for light direction, intensities, and color scale.
 - Updates camera position and orientation relative to ISS.
 - Renders the scene and refreshes gizmo labels.
 
 ## Files of Interest
 - `src/core/earth/src/main.ts`: All scene, shader, and UI logic.
 - `src/core/earth/package.json`: Vite + Three.js setup.
 - `src/core/earth/start.sh`: Starts the dev server.
