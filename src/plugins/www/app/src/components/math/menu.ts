import { Menu } from "../util/menu";

// Define the interface for the visualization host to avoid circular dependency if possible,
// or just use any/loose typing if acceptable. For better type safety:
export interface MathConfigHost {
    cameraOrbitRadius: number;
    cameraHeight: number;
    cameraHeightOsc: number;
    cameraHeightSpeed: number;
    cameraRoll: number;
    cameraRollSpeed: number;
    cameraOrbitSpeed: number;
    cameraLookAtY: number;

    curveA: number;
    curveB: number;
    curveC: number;
    curveD: number;
    curveE: number;
    curveF: number;

    gridOpacity: number;
    gridOpacityOsc: number;
    gridOscSpeed: number;

    innerOrbitSpeed: number;
    middleOrbitSpeed: number;
    outerOrbitSpeed: number;

    buildConfigSnapshot: () => any;
}

export function setupMathMenu(viz: MathConfigHost) {
    const menu = new Menu("math-config-panel", "Menu");

    menu.addHeader("Camera");
    menu.addSlider("Radius", viz.cameraOrbitRadius, 8, 35, 0.5, (v) => (viz.cameraOrbitRadius = v));
    menu.addSlider("Height", viz.cameraHeight, -5, 15, 0.5, (v) => (viz.cameraHeight = v));
    menu.addSlider("Height Osc", viz.cameraHeightOsc, 0, 6, 0.1, (v) => (viz.cameraHeightOsc = v));
    menu.addSlider("Height Speed", viz.cameraHeightSpeed, 0, 2, 0.05, (v) => (viz.cameraHeightSpeed = v));
    menu.addSlider("Roll", viz.cameraRoll, -0.6, 0.6, 0.01, (v) => (viz.cameraRoll = v));
    menu.addSlider("Roll Speed", viz.cameraRollSpeed, 0, 1, 0.01, (v) => (viz.cameraRollSpeed = v));
    menu.addSlider("Orbit Speed", viz.cameraOrbitSpeed, 0, 0.02, 0.0005, (v) => (viz.cameraOrbitSpeed = v), (v) => v.toFixed(4));
    menu.addSlider("Look Y", viz.cameraLookAtY, -5, 5, 0.5, (v) => (viz.cameraLookAtY = v));

    menu.addHeader("Curve Shape");
    menu.addSlider("Curve A", viz.curveA, -3, 3, 0.05, (v) => (viz.curveA = v));
    menu.addSlider("Curve B", viz.curveB, -3, 3, 0.05, (v) => (viz.curveB = v));
    menu.addSlider("Curve C", viz.curveC, -3, 3, 0.05, (v) => (viz.curveC = v));
    menu.addSlider("Curve D", viz.curveD, -3, 3, 0.05, (v) => (viz.curveD = v));
    menu.addSlider("Curve E", viz.curveE, -3, 3, 0.05, (v) => (viz.curveE = v));
    menu.addSlider("Curve F", viz.curveF, -3, 3, 0.05, (v) => (viz.curveF = v));

    menu.addHeader("Grid");
    menu.addSlider("Opacity", viz.gridOpacity, 0, 1, 0.05, (v) => (viz.gridOpacity = v));
    menu.addSlider("Oscillation", viz.gridOpacityOsc, 0, 0.7, 0.01, (v) => (viz.gridOpacityOsc = v));
    menu.addSlider("Osc Speed", viz.gridOscSpeed, 0, 2, 0.05, (v) => (viz.gridOscSpeed = v));

    menu.addHeader("Orbits");
    menu.addSlider("Inner Speed", viz.innerOrbitSpeed, 0, 0.01, 0.0005, (v) => (viz.innerOrbitSpeed = v), (v) => v.toFixed(4));
    menu.addSlider("Middle Speed", viz.middleOrbitSpeed, 0, 0.01, 0.0005, (v) => (viz.middleOrbitSpeed = v), (v) => v.toFixed(4));
    menu.addSlider("Outer Speed", viz.outerOrbitSpeed, 0, 0.01, 0.0005, (v) => (viz.outerOrbitSpeed = v), (v) => v.toFixed(4));

    menu.addButton("Copy Config", () => {
        const payload = JSON.stringify(viz.buildConfigSnapshot(), null, 2);
        navigator.clipboard?.writeText(payload);
    }, true);

    return {
        dispose: () => menu.dispose(),
        setToggleVisible: (visible: boolean) => menu.setToggleVisible(visible),
    };
}
