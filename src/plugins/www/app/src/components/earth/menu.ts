import { Menu } from "../util/menu";
import { ProceduralOrbit } from "./index";

export function setupEarthMenu(orbit: ProceduralOrbit) {
    const menu = new Menu("earth-config-panel", "Menu");

    menu.addHeader("Rotation");

    const toPeriodSeconds = (rotSpeedRadPerSec: number) => {
        if (!isFinite(rotSpeedRadPerSec) || rotSpeedRadPerSec <= 0) return Infinity;
        return (Math.PI * 2) / rotSpeedRadPerSec;
    };
    const toRotSpeed = (periodSeconds: number) => {
        if (!isFinite(periodSeconds) || periodSeconds <= 0) return 0;
        return (Math.PI * 2) / periodSeconds;
    };

    menu.addSlider(
        "Earth",
        Math.min(60, toPeriodSeconds(orbit.earthRotSpeed)),
        1,
        60,
        1,
        (v) => (orbit.earthRotSpeed = toRotSpeed(v)),
        (v) => (isFinite(v) ? v.toFixed(0) : "∞")
    );

    menu.addSlider(
        "Sun Orbit",
        orbit.sunOrbitSpeed,
        0,
        0.005,
        0.0001,
        (v) => (orbit.sunOrbitSpeed = v),
        (v) => v.toFixed(4)
    );

    menu.addSlider(
        "Sun Pos",
        orbit.sunOrbitAngleRad,
        0,
        Math.PI * 2,
        0.01,
        (v) => orbit.setSunOrbitAngleRad(v),
        (v) => v.toFixed(2)
    );

    menu.addHeader("Atmosphere");

    menu.addSlider(
        "Cloud Amt",
        orbit.cloudAmount,
        0,
        1,
        0.01,
        (v) => (orbit.cloudAmount = v),
        (v) => v.toFixed(2)
    );

    menu.addSlider(
        "Brightness",
        orbit.cloudBrightness,
        0,
        5,
        0.1,
        (v) => (orbit.cloudBrightness = v),
        (v) => v.toFixed(1)
    );

    menu.addHeader("Cloud Layer 1");
    menu.addSlider(
        "Speed",
        orbit.cloud1RotSpeed * 100000,
        0,
        50,
        1,
        (v) => (orbit.cloud1RotSpeed = v / 100000),
        (v) => v.toFixed(0)
    );
    menu.addSlider(
        "Opacity",
        orbit.cloud1Opacity,
        0.5,
        1,
        0.01,
        (v) => (orbit.cloud1Opacity = v),
        (v) => v.toFixed(2)
    );

    menu.addHeader("Cloud Layer 2");
    menu.addSlider(
        "Speed",
        orbit.cloud2RotSpeed * 100000,
        0,
        50,
        1,
        (v) => (orbit.cloud2RotSpeed = v / 100000),
        (v) => v.toFixed(0)
    );
    menu.addSlider(
        "Opacity",
        orbit.cloud2Opacity,
        0.5,
        1,
        0.01,
        (v) => (orbit.cloud2Opacity = v),
        (v) => v.toFixed(2)
    );

    menu.addHeader("Camera");
    menu.addSlider(
        "Distance",
        orbit.cameraDistance,
        0,
        30,
        0.5,
        (v) => (orbit.cameraDistance = v),
        (v) => v.toFixed(1)
    );
    menu.addSlider(
        "Yaw",
        orbit.cameraYaw,
        0,
        Math.PI * 2,
        0.01,
        (v) => (orbit.cameraYaw = v),
        (v) => v.toFixed(2)
    );
    menu.addSlider(
        "Orbit",
        orbit.cameraOrbit,
        0,
        Math.PI * 2,
        0.01,
        (v) => (orbit.cameraOrbit = v),
        (v) => v.toFixed(2)
    );

    menu.addButton("Copy Config", () => {
        const payload = JSON.stringify(orbit.buildConfigSnapshot(), null, 2);
        navigator.clipboard?.writeText(payload);
    }, true);

    return {
        dispose: () => menu.dispose(),
        setToggleVisible: (visible: boolean) => menu.setToggleVisible(visible),
    };
}
