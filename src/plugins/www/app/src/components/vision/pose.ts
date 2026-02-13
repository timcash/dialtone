export class PoseTracker {
  private video: HTMLVideoElement;
  private poseLandmarker: any = null;
  private stream: MediaStream | null = null;
  private lastVideoTime = -1;

  constructor() {
    this.video = document.createElement("video");
    this.video.autoplay = true;
    this.video.playsInline = true;
    this.video.style.display = "none";
    document.body.appendChild(this.video);
  }

  async init() {
    // Load MediaPipe from CDN to ensure it works without complex local setup
    // @ts-ignore
    const vision = await import("https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@0.10.15/vision_bundle.mjs");
    const { PoseLandmarker, FilesetResolver } = vision;

    const filesetResolver = await FilesetResolver.forVisionTasks(
      "https://cdn.jsdelivr.net/npm/@mediapipe/tasks-vision@0.10.15/wasm"
    );

    this.poseLandmarker = await PoseLandmarker.createFromOptions(filesetResolver, {
      baseOptions: {
        modelAssetPath: `https://storage.googleapis.com/mediapipe-models/pose_landmarker/pose_landmarker_lite/float16/1/pose_landmarker_lite.task`,
        delegate: "GPU"
      },
      runningMode: "VIDEO",
      numPoses: 1
    });
  }

  async startCamera() {
    if (this.stream) return;
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ 
        video: { width: 640, height: 480 } 
      });
      this.video.srcObject = this.stream;
      await this.video.play();
    } catch (err) {
      console.error("Error accessing webcam:", err);
    }
  }

  stopCamera() {
    if (this.stream) {
      this.stream.getTracks().forEach(track => track.stop());
      this.stream = null;
    }
  }

  getLandmarks() {
    if (!this.poseLandmarker || !this.video.readyState || this.video.paused) {
      return null;
    }

    const startTimeMs = performance.now();
    if (this.lastVideoTime !== this.video.currentTime) {
      this.lastVideoTime = this.video.currentTime;
      const result = this.poseLandmarker.detectForVideo(this.video, startTimeMs);
      if (result.landmarks && result.landmarks.length > 0) {
        // Return normalized landmarks (0-1 range) which applyPose expects
        return result.landmarks[0];
      }
    }
    return null;
  }

  get isCameraOn() {
    return !!this.stream;
  }
}
