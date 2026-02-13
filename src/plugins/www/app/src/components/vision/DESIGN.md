# Vision Component: Human Body Tracking

## Goal
Implement a real-time human body pose estimation visualization using Three.js and browser-based machine learning. The component will render a 3D "skeleton" of the user in real-time, overlaying it on a 3D scene.

## Features
- **Real-time Pose Detection**: Use a pre-trained model (e.g., MediaPipe Pose or Transformers.js) to detect human keypoints from the camera.
- **Three.js Visualization**: Render the detected keypoints as 3D spheres and the connections (bones) as glowing 3D lines.
- **Interactive Menu**:
    - `Camera On/Off`: Toggle the webcam stream.
    - `Body Track On/Off`: Toggle the machine learning inference.
    - `Track Demo`: Play a pre-recorded or simulated pose animation to show off the visualization without a camera.
- **Marketing Text**: High-impact messaging about computer vision and human-machine interaction.

## Technical Details

### Pose Detection
- **Model**: MediaPipe Pose (via `@mediapipe/tasks-vision`) is the primary candidate for high-performance, GPU-accelerated browser pose estimation.
- **Inference**: Keypoints (landmarks) are extracted in 3D (x, y, z).
- **Mapping**: Map normalized MediaPipe coordinates to Three.js world space.

### Visualization Style
- **Cyber-Skeleton**: Glowing neon lines for the skeletal structure.
- **Depth and Scale**: Use the depth (Z) data from the model to create a true 3D effect.
- **Bloom Pass**: Intense post-processing glow consistent with the Dialtone aesthetic.

### Component Structure
- `mountVision(container: HTMLElement)`: Entry point, handles UI and marketing text.
- `VisionVisualization`: Class managing the Three.js scene, camera, and skeletal rendering.
- `PoseTracker`: Class handling the webcam stream and MediaPipe Pose Landmarker.
- `menu.ts`: UI controls integration.

## Marketing Copy
- **Title**: Bio-Digital Integration
- **Subtitle**: Turning human motion into machine-readable geometry.
- **Description**: Vision uses low-latency pose estimation to bridge the gap between physical action and digital intent.
