# Earth Visualization Component

## Recent Improvements (February 2026)

### 1. Scene Scaling (10x)
- **Objective**: Improve rendering precision and reduce z-fighting.
- **Changes**:
    - `earthRadius`: 5 -> 50
    - `cameraDistance`: 6 -> 60
    - All other distances and objects scaled proportionally.

### 2. Layering & Visibility
- **Objective**: Ensure clouds are visible above the land, but allowing land to be seen through gaps.
- **Changes**:
    - **Cloud Layers**: Moved to `1.2` (Cloud 1) and `1.5` (Cloud 2) above surface (Land is `0.6`).
    - **Transparency**: Disabled `depthWrite` in cloud material to prevent occlusion of the land layer behind transparent parts of clouds.
    - **Opacity**: Increased to `0.95` (Cloud 1) and `0.90` (Cloud 2) to provide solid occlusion where clouds exist.

### 3. Visual Refinements
- **Larger Patterns**: Cloud noise scale reduced by 5x (0.04/0.1) for larger, more realistic formations.
- **Faster Motion**: Oscillation and rotation speeds increased by 5x for more dynamic visuals.
- **Lighting**: Boosted `Sun Intensity` (1.0) and `Ambient Intensity` (0.5) to ensure clouds remain visible on the dark side of the Earth.

### 4. Controls
- **New Sliders**:
    - **Land Radius**: Adjust land layer height (50.0 - 55.0).
    - **Cloud 1 Radius**: Adjust first cloud layer height (50.0 - 60.0).
    - **Cloud 2 Radius**: Adjust second cloud layer height (50.0 - 60.0).
