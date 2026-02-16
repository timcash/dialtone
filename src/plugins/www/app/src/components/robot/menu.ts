import { Menu } from "../util/menu";

// Define a subset of RobotArmVisualization to avoid circular imports if possible,
// or just use loose typing.
// We need access to: targetPosition, robotArm, etc.
interface RobotVizLike {
    targetPosition: { x: number; y: number; z: number };
    robotArm: {
        joints: { getAngle: () => number; setAngle: (v: number) => void }[];
        setGrip: (v: number) => void;
    };
    autoAnimate: boolean;
    pickNewTarget: () => void;
    cameraRadius: number;
    cameraHeight: number;
    cameraOrbitSpeed: number;
    targetMoveInterval: number; // Not used in menu?
    // Head/Tilt are likely just joints?
    // In config.ts, joints were: Base(Y), Shoulder(Z), Elbow(Y), Forearm(Z), Wrist(Z).
    // 5 joints.
    // In `menu.ts` I had properties like `headPan`, `headTilt`.
    // Looking at `robot/config.ts`, it just had 5 joints.
    // My `robot/menu.ts` seemed to hallucinate `targetPitch`, `targetRoll`, `headPan`?
    // `robot/config.ts` (1238) had "Joint Angles" with 5 sliders.
    // And "Target" with X, Y, Z.
    // And "Gripper".

    // So I should revert `robot/menu.ts` to match `robot/config.ts` structure.
}

export function setupRobotMenu(viz: RobotVizLike): void {
    const menu = Menu.getInstance();

    menu.addHeader("Kinematic Solver");
    // Checkbox for Auto Track
    // Menu doesn't have addCheckbox?
    // `robot/config.ts` had `addCheckbox`.
    // I need to check `util/menu.ts` to see what it supports.
    // If no checkbox, I can use a button that toggles? "Auto: ON" / "Auto: OFF".

    const autoBtn = menu.addButton(`Auto Track: ${viz.autoAnimate ? "ON" : "OFF"}`, () => {
        viz.autoAnimate = !viz.autoAnimate;
        autoBtn.textContent = `Auto Track: ${viz.autoAnimate ? "ON" : "OFF"}`;
    }, true);

    menu.addButton("New Target", () => viz.pickNewTarget(), true);

    menu.addHeader("Camera");
    menu.addSlider("Distance", viz.cameraRadius, 6, 20, 0.5, (v) => (viz.cameraRadius = v), (v) => v.toFixed(1));
    menu.addSlider("Height", viz.cameraHeight, 1, 12, 0.5, (v) => (viz.cameraHeight = v), (v) => v.toFixed(1));
    menu.addSlider("Orbit Speed", viz.cameraOrbitSpeed, 0, 0.5, 0.01, (v) => (viz.cameraOrbitSpeed = v), (v) => v.toFixed(2));

    menu.addHeader("Target");
    // We need to update targetLine when these change?
    // `viz` might have `updateTargetLine`?
    // `robot/config.ts` called `viz.updateTargetLine()`.
    // I should add `updateTargetLine` to interface.
    const updateTarget = (axis: "x" | "y" | "z", v: number) => {
        viz.targetPosition[axis] = v;
        (viz as any).updateTargetLine?.();
    };

    menu.addSlider("Target X", viz.targetPosition.x, -4, 4, 0.1, (v) => updateTarget("x", v), (v) => v.toFixed(1));
    menu.addSlider("Target Y", viz.targetPosition.y, -2, 6, 0.1, (v) => updateTarget("y", v), (v) => v.toFixed(1));
    menu.addSlider("Target Z", viz.targetPosition.z, -4, 4, 0.1, (v) => updateTarget("z", v), (v) => v.toFixed(1));

    menu.addHeader("Joint Angles");
    const jointConfigs = [
        { name: "Base (Y)", min: -180, max: 180 },
        { name: "Shoulder (Z)", min: -100, max: 100 },
        { name: "Elbow (Y)", min: -180, max: 180 },
        { name: "Forearm (Z)", min: -100, max: 100 },
        { name: "Wrist (Z)", min: -100, max: 100 },
    ];

    jointConfigs.forEach((config, i) => {
        // We need to read current angle?
        // viz.robotArm.joints[i].getAngle()
        // `Menu` usually takes initial value.
        const initial = viz.robotArm.joints[i].getAngle();
        menu.addSlider(
            config.name,
            initial,
            config.min,
            config.max,
            1,
            (v) => {
                viz.robotArm.joints[i].setAngle(v);
                viz.autoAnimate = false;
                autoBtn.textContent = "Auto Track: OFF";
            },
            (v) => `${Math.round(v)}°`
        );
    });

    menu.addHeader("Gripper");
    menu.addSlider("Grip", 0.5, 0, 1, 0.01, (v) => viz.robotArm.setGrip(v), (v) => `${Math.round(v * 100)}%`);


}
