import { Menu } from "../util/menu";

type AboutLightConfig = {
    count: number;
    dwell: number;
    wander: number;
    seed: number;
    brightness: number;
};

type AboutSparkConfig = {
    intervalSeconds: number;
    pauseMs: number;
    drainRatePerMs: number;
};

type AboutPowerConfig = {
    maxPower: number;
    regenPerSec: number;
    restThreshold: number;
};

type AboutMotionConfig = {
    glideSpeed: number;
    glideAccel: number;
};

type AboutViz = {
    setLightCount: (value: number) => void;
    setDwellSeconds: (value: number) => void;
    setWanderDistance: (value: number) => void;
    setSeed: (value: number) => void;
    setBrightness: (value: number) => void;
    setStepIntervalMs: (value: number) => void;
    setSparkIntervalSeconds: (value: number) => void;
    setSparkPauseMs: (value: number) => void;
    setSparkDrainRatePerMs: (value: number) => void;
    setMaxPower: (value: number) => void;
    setPowerRegenRatePerSec: (value: number) => void;
    setRestThreshold: (value: number) => void;
    setGlideSpeed: (value: number) => void;
    setGlideAccel: (value: number) => void;
};

type AboutConfigOptions = {
    viz: AboutViz;
    lightConfig: AboutLightConfig;
    sparkConfig: AboutSparkConfig;
    powerConfig: AboutPowerConfig;
    motionConfig: AboutMotionConfig;
};

export function setupAboutMenu(options: AboutConfigOptions): () => void {
    const { viz, lightConfig, sparkConfig, powerConfig, motionConfig } = options;
    let presetSwapTimer: number | null = null;
    const presetSwapMs = 5000;

    const menu = Menu.getInstance();
    menu.clear();

    // Presets Data
    const lightPresets = [
        { label: "Dim", count: 3, brightness: 0.9, maxPower: 4, regenPerSec: 0.7, restThreshold: 0.8 },
        { label: "Balanced", count: 4, brightness: 1.35, maxPower: 5, regenPerSec: 1, restThreshold: 1 },
        { label: "Bright", count: 5, brightness: 1.7, maxPower: 6, regenPerSec: 1.3, restThreshold: 1.2 },
        { label: "Hot", count: 6, brightness: 2.1, maxPower: 7, regenPerSec: 1.8, restThreshold: 1.4 },
    ];
    const motionPresets = [
        { label: "Floaty", glideSpeed: 4.5, glideAccel: 5.5, wander: 6, dwell: 10 },
        { label: "Steady", glideSpeed: 7, glideAccel: 8, wander: 8, dwell: 8 },
        { label: "Agile", glideSpeed: 10, glideAccel: 12, wander: 10, dwell: 6 },
        { label: "Frenzy", glideSpeed: 12, glideAccel: 14, wander: 12, dwell: 5 },
    ];
    const sparkPresets = [
        { label: "Rare", intervalSeconds: 7, pauseMs: 1500, drainRatePerMs: 0.0006 },
        { label: "Normal", intervalSeconds: 4, pauseMs: 1200, drainRatePerMs: 0.001 },
        { label: "Active", intervalSeconds: 2.5, pauseMs: 900, drainRatePerMs: 0.0013 },
        { label: "Storm", intervalSeconds: 1.5, pauseMs: 700, drainRatePerMs: 0.0016 },
    ];

    let lightPresetIndex = 3; // Hot
    let motionPresetIndex = 3; // Frenzy
    let sparkPresetIndex = 3; // Storm
    let presetSwapEnabled = 0;

    // Sliders references for syncing
    let lightSlider: { setValue: (v: number) => void };
    let motionSlider: { setValue: (v: number) => void };
    let sparkSlider: { setValue: (v: number) => void };

    const applyLightPreset = (index: number, syncUi = false) => {
        const preset = lightPresets[index];
        lightConfig.count = preset.count;
        lightConfig.brightness = preset.brightness;
        powerConfig.maxPower = preset.maxPower;
        powerConfig.regenPerSec = preset.regenPerSec;
        powerConfig.restThreshold = preset.restThreshold;
        viz.setLightCount(preset.count);
        viz.setBrightness(preset.brightness);
        viz.setMaxPower(preset.maxPower);
        viz.setPowerRegenRatePerSec(preset.regenPerSec);
        viz.setRestThreshold(preset.restThreshold);
        if (syncUi && lightSlider) lightSlider.setValue(index);
    };
    const applyMotionPreset = (index: number, syncUi = false) => {
        const preset = motionPresets[index];
        motionConfig.glideSpeed = preset.glideSpeed;
        motionConfig.glideAccel = preset.glideAccel;
        lightConfig.wander = preset.wander;
        lightConfig.dwell = preset.dwell;
        viz.setGlideSpeed(preset.glideSpeed);
        viz.setGlideAccel(preset.glideAccel);
        viz.setWanderDistance(preset.wander);
        viz.setDwellSeconds(preset.dwell);
        if (syncUi && motionSlider) motionSlider.setValue(index);
    };
    const applySparkPreset = (index: number, syncUi = false) => {
        const preset = sparkPresets[index];
        sparkConfig.intervalSeconds = preset.intervalSeconds;
        sparkConfig.pauseMs = preset.pauseMs;
        sparkConfig.drainRatePerMs = preset.drainRatePerMs;
        viz.setSparkIntervalSeconds(preset.intervalSeconds);
        viz.setSparkPauseMs(preset.pauseMs);
        viz.setSparkDrainRatePerMs(preset.drainRatePerMs);
        if (syncUi && sparkSlider) sparkSlider.setValue(index);
    };

    // Init logic
    applyLightPreset(lightPresetIndex);
    applyMotionPreset(motionPresetIndex);
    applySparkPreset(sparkPresetIndex);

    // UI Building
    menu.addHeader("Presets");

    lightSlider = menu.addSlider("Light", lightPresetIndex, 0, lightPresets.length - 1, 1, (v) => {
        lightPresetIndex = Math.round(v);
        applyLightPreset(lightPresetIndex);
    }, (v) => lightPresets[Math.round(v)]?.label ?? `${v}`);

    motionSlider = menu.addSlider("Motion", motionPresetIndex, 0, motionPresets.length - 1, 1, (v) => {
        motionPresetIndex = Math.round(v);
        applyMotionPreset(motionPresetIndex);
    }, (v) => motionPresets[Math.round(v)]?.label ?? `${v}`);

    sparkSlider = menu.addSlider("Spark", sparkPresetIndex, 0, sparkPresets.length - 1, 1, (v) => {
        sparkPresetIndex = Math.round(v);
        applySparkPreset(sparkPresetIndex);
    }, (v) => sparkPresets[Math.round(v)]?.label ?? `${v}`);

    menu.addSlider("Swap", presetSwapEnabled, 0, 1, 1, (v) => {
        presetSwapEnabled = Math.round(v);
        if (presetSwapEnabled && presetSwapTimer === null) {
            presetSwapTimer = window.setInterval(() => {
                lightPresetIndex = (lightPresetIndex + 1) % lightPresets.length;
                motionPresetIndex = (motionPresetIndex + 1) % motionPresets.length;
                sparkPresetIndex = (sparkPresetIndex + 1) % sparkPresets.length;
                applyLightPreset(lightPresetIndex, true);
                applyMotionPreset(motionPresetIndex, true);
                applySparkPreset(sparkPresetIndex, true);
            }, presetSwapMs);
        } else if (!presetSwapEnabled && presetSwapTimer !== null) {
            window.clearInterval(presetSwapTimer);
            presetSwapTimer = null;
        }
    }, (v) => (Math.round(v) === 1 ? "On" : "Off"));

    menu.addHeader("Seed");
    menu.addSlider("Seed", lightConfig.seed, 1, 9999, 1, (v) => {
        const seed = Math.round(v);
        lightConfig.seed = seed;
        viz.setSeed(seed);
    });
    menu.addSlider("Dwell (s)", lightConfig.dwell, 2, 15, 1, (v) => {
        lightConfig.dwell = v;
        viz.setDwellSeconds(v);
    });
    menu.addSlider("Wander", lightConfig.wander, 2, 16, 1, (v) => {
        lightConfig.wander = v;
        viz.setWanderDistance(v);
    });

    return () => {
        if (presetSwapTimer !== null) {
            window.clearInterval(presetSwapTimer);
            presetSwapTimer = null;
        }
    };
}
