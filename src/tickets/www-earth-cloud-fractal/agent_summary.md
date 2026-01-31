# Walkthrough: Modular Cinematic Earth Visualization

I've completed the refinement of the Earth viewing experience and transitioned the codebase to a modular architecture for better maintainability.

## Visual & Mathematical Refinements

### 1. Grand Orbital Scale
- **Extreme Altitude**: Increased the orbital height to **2230 KM** (`height: 2.8`), providing a grand, "God-eye" perspective of the planet's curvature.
- **Cinematic Framing**: Adjusted the camera path to provide dramatic "look down" views of both the ISS and the Earth silhoutte.

### 2. ISS Polish
- **Material Realism**: Switched the ISS body to a polished aluminum finish (white/grey with high metalness) and updated the solar panels to a reflective grey. This eliminates the "brown" appearance and aligns with a premium orbital aesthetic.
- **Unified Lighting**: Refined the scene lighting (Hemisphere and Point sources) to accentuate the metallic highlights of the station.

### 3. Horizon-Tangent Focus
- **Mathematical Lock**: Implemented a robust constraint that calculates the camera's world-space gaze and clamps it to the Earth's horizon disk. 
- **Space-Free Viewing**: This mathematically guarantees that the planetary sphere remains the focal point of every frame, preventing the camera from ever looking out into empty space during extreme pans.

### 4. Lively Atmosphere
- **Fractal Brownian Motion (FBM)**: Replaced simple noise with 4-octave fractal patterns, providing rich organic detail and varied cloud structures.
- **Domain Warping**: Implemented coordinate distortion to create dynamic "swirling" atmospheric patterns.
- **Atmospheric Breathing**: Added a low-frequency oscillation to the cloud thickness (`sin(uTime * 0.12)`), creating a living, pulsing atmosphere.

````carousel
![Initial Fractal Clouds](file:///home/user/.gemini/antigravity/brain/8fc86a49-4f01-4c00-9e26-3fc9e0934bef/earth_clouds_initial_1769827133973.png)
<!-- slide -->
![Clouds After 15s (Pulsing)](file:///home/user/.gemini/antigravity/brain/8fc86a49-4f01-4c00-9e26-3fc9e0934bef/earth_clouds_after_15s_1769827153196.png)
````

## Technical Refactoring

To keep `earth.ts` maintainable, I've extracted specialized logic into a new `components/earth/` directory:

- **[iss_model.ts](file:///home/user/dialtone/src/plugins/www/app/src/components/earth/iss_model.ts)**: Encapsulates the station geometry and material definitions.
- **[camera_math.ts](file:///home/user/dialtone/src/plugins/www/app/src/components/earth/camera_math.ts)**: Houses the horizon focus clamping logic.
- **[config_ui.ts](file:///home/user/dialtone/src/plugins/www/app/src/components/earth/config_ui.ts)**: Manages the telemetry display and configuration sliders.

## Final Verification Results

- **Stability**: Automated `chromedp` tests confirm the camera gaze remains locked within the Earth's silhouette across the entire orbital cycle.
- **Aesthetics**: Manual inspection confirms the reflective grey ISS aesthetic and the grand planetary scale are functioning as intended.
- **Maintainability**: `earth.ts` is now focused purely on scene orchestration, making it significantly easier to work with.
