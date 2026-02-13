import { Menu } from "../util/menu";

type VisionConfigOptions = {
    isCameraOn: boolean;
    toggleCamera: () => void;
    isTracking: boolean;
    toggleTracking: () => void;
    isDemo: boolean;
    toggleDemo: () => void;
    jointSize: number;
    onJointSizeChange: (v: number) => void;
    boneWidth: number;
    onBoneWidthChange: (v: number) => void;
    bloomStrength: number;
    onBloomStrengthChange: (v: number) => void;
    color: number;
    onColorChange: (v: number) => void;
    cameraDistance: number;
    onCameraDistanceChange: (v: number) => void;
};

export function setupVisionMenu(options: VisionConfigOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

    menu.addHeader("Bio-Digital Integration");

    menu.addButton(options.isCameraOn ? "Camera Off" : "Camera On", () => {
        options.toggleCamera();
    }, options.isCameraOn);

    menu.addButton(options.isTracking ? "Stop Tracking" : "Body Track", () => {
        options.toggleTracking();
    }, options.isTracking);

    menu.addButton(options.isDemo ? "Stop Demo" : "Track Demo", () => {
        options.toggleDemo();
    }, options.isDemo);

    menu.addHeader("Presets");
    const presets = [
        { label: "Cyan Neon", color: 0x00ffff, joint: 0.04, bone: 3, bloom: 1.5 },
        { label: "Pink Ghost", color: 0xff00ff, joint: 0.02, bone: 1, bloom: 2.5 },
        { label: "Gold Armor", color: 0xffd700, joint: 0.08, bone: 8, bloom: 0.8 },
        { label: "Matrix", color: 0x00ff00, joint: 0.03, bone: 4, bloom: 1.2 },
    ];

    presets.forEach(p => {
        menu.addButton(p.label, () => {
            options.onColorChange(p.color);
            options.onJointSizeChange(p.joint);
            options.onBoneWidthChange(p.bone);
            options.onBloomStrengthChange(p.bloom);
            // We need to trigger UI updates for the sliders too, but Menu doesn't easily support external updates to slider values without holding references.
            // For now, these presets will update the visualization.
        });
    });

    menu.addHeader("Geometry");
    menu.addSlider("Joint Size", options.jointSize, 0.01, 0.2, 0.01, options.onJointSizeChange, v => v.toFixed(2));
    menu.addSlider("Bone Width", options.boneWidth, 1, 20, 1, options.onBoneWidthChange);
    menu.addSlider("Bloom", options.bloomStrength, 0, 3, 0.1, options.onBloomStrengthChange, v => v.toFixed(1));
    menu.addSlider("Camera Dist", options.cameraDistance, 2, 20, 0.5, options.onCameraDistanceChange, v => v.toFixed(1));

    menu.addHeader("Status");
    if (options.isTracking && options.isCameraOn) {
        menu.addStatus().update("Live Tracking Active");
    } else if (options.isDemo) {
        menu.addStatus().update("Demo Mode Active");
    } else {
        menu.addStatus().update("Ready");
    }
}
