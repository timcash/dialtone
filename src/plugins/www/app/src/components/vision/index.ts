import { startTyping } from "../util/typing";
import { setupVisionMenu } from "./menu";
import { VisionVisualization } from "./visualization";
import { PoseTracker } from "./pose";

export function mountVision(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Vision section: body tracking">
      <h2 data-typing-title>Bio-Digital Integration</h2>
      <p data-typing-subtitle></p>
    </div>
  `;

  const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement | null;
  const subtitles = [
    "Human motion translated to machine geometry.",
    "Real-time pose estimation and 3D reconstruction.",
    "Bridging the gap between physical and digital intent.",
    "Low-latency neural inference in the browser.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new VisionVisualization(container);
  const tracker = new PoseTracker();
  let isTracking = false;
  let isDemo = true; // Start with demo on for immediate visual feedback
  let isVisible = true;

  viz.setDemo(isDemo);
  tracker.init();

  const updateMenu = () => {
    setupVisionMenu({
      isCameraOn: tracker.isCameraOn,
      toggleCamera: async () => {
        if (tracker.isCameraOn) {
          tracker.stopCamera();
        } else {
          isDemo = false;
          viz.setDemo(false);
          await tracker.startCamera();
        }
        updateMenu();
      },
      isTracking: isTracking,
      toggleTracking: () => {
        isTracking = !isTracking;
        if (isTracking) {
            isDemo = false;
            viz.setDemo(false);
        }
        updateMenu();
      },
      isDemo: isDemo,
      toggleDemo: () => {
        isDemo = !isDemo;
        viz.setDemo(isDemo);
        if (isDemo) {
            isTracking = false;
            tracker.stopCamera();
        }
        updateMenu();
      },
      jointSize: viz.jointSize,
      onJointSizeChange: (v) => viz.setJointSize(v),
      boneWidth: viz.boneWidth,
      onBoneWidthChange: (v) => viz.setBoneWidth(v),
      bloomStrength: 1.5, // Default
      onBloomStrengthChange: (v) => viz.setBloomStrength(v),
      color: viz.color,
      onColorChange: (v) => viz.setColor(v),
      cameraDistance: viz.cameraDistance,
      onCameraDistanceChange: (v) => viz.setCameraDistance(v)
    });
  };

  // Dedicated loop for ML tracking (decoupled from render loop)
  const mlLoop = () => {
    if (!isVisible) {
        setTimeout(mlLoop, 500); // Check less frequently when hidden
        return;
    }
    if (isTracking && tracker.isCameraOn) {
      const landmarks = tracker.getLandmarks();
      if (landmarks) {
        viz.updatePose(landmarks);
      }
    }
    requestAnimationFrame(mlLoop);
  };
  mlLoop();

  return {
    dispose: () => {
      viz.dispose();
      tracker.stopCamera();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      const wasVisible = isVisible;
      isVisible = visible;
      viz.setVisible(visible);
      
      if (visible && !wasVisible) {
          // Becoming visible
          if (isTracking && !tracker.isCameraOn) {
              tracker.startCamera().then(() => updateMenu());
          }
      } else if (!visible && wasVisible) {
          // Becoming invisible
          tracker.stopCamera();
          isDemo = false;
          viz.setDemo(false);
          updateMenu();
      }
    },
    updateUI: () => {
      updateMenu();
    }
  };
}
