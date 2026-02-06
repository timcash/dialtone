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

export function setupAboutConfig(options: AboutConfigOptions) {
  const { viz, lightConfig, sparkConfig, powerConfig, motionConfig } = options;
  let presetSwapTimer: number | null = null;

  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "about-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById("about-config-panel") as HTMLDivElement | null;
  if (panel && toggle) {
    const setOpen = (open: boolean) => {
      panel.hidden = !open;
      panel.style.display = open ? "grid" : "none";
      toggle.setAttribute("aria-expanded", String(open));
    };
    setOpen(false);
    toggle.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      setOpen(panel.hidden);
    });

    panel.classList.add("about-config-panel");
    const addHeader = (text: string) => {
      const header = document.createElement("h3");
      header.textContent = text;
      panel.appendChild(header);
    };
    const sliderRegistry: Record<
      string,
      { slider: HTMLInputElement; valueEl: HTMLSpanElement }
    > = {};
    const addSlider = (
      label: string,
      min: number,
      max: number,
      step: number,
      value: number,
      onInput: (v: number) => void,
      format: (v: number) => string = (v) => v.toFixed(0),
      key?: string,
    ) => {
      const row = document.createElement("div");
      row.className = "earth-config-row about-config-row";
      const labelWrap = document.createElement("label");
      const sliderId = `about-slider-${(key ?? label)
        .replace(/\s+/g, "-")
        .toLowerCase()}`;
      labelWrap.className = "earth-config-label";
      labelWrap.htmlFor = sliderId;
      labelWrap.textContent = label;
      const slider = document.createElement("input");
      slider.type = "range";
      slider.id = sliderId;
      slider.min = `${min}`;
      slider.max = `${max}`;
      slider.step = `${step}`;
      slider.value = `${value}`;
      row.appendChild(labelWrap);
      row.appendChild(slider);
      const valueEl = document.createElement("span");
      valueEl.className = "earth-config-value";
      valueEl.textContent = format(value);
      row.appendChild(valueEl);
      panel.appendChild(row);
      slider.addEventListener("input", () => {
        const v = parseFloat(slider.value);
        onInput(v);
        valueEl.textContent = format(v);
      });
      if (key) {
        sliderRegistry[key] = { slider, valueEl };
      }
    };
    const setSliderValue = (
      key: string,
      value: number,
      format?: (v: number) => string,
    ) => {
      const entry = sliderRegistry[key];
      if (!entry) return;
      entry.slider.value = `${value}`;
      entry.valueEl.textContent = format ? format(value) : value.toFixed(0);
    };

    addHeader("Presets");
    const lightPresets = [
      {
        label: "Dim",
        count: 3,
        brightness: 0.9,
        maxPower: 4,
        regenPerSec: 0.7,
        restThreshold: 0.8,
      },
      {
        label: "Balanced",
        count: 4,
        brightness: 1.35,
        maxPower: 5,
        regenPerSec: 1,
        restThreshold: 1,
      },
      {
        label: "Bright",
        count: 5,
        brightness: 1.7,
        maxPower: 6,
        regenPerSec: 1.3,
        restThreshold: 1.2,
      },
      {
        label: "Hot",
        count: 6,
        brightness: 2.1,
        maxPower: 7,
        regenPerSec: 1.8,
        restThreshold: 1.4,
      },
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
    let lightPresetIndex = 1;
    let motionPresetIndex = 1;
    let sparkPresetIndex = 1;
    let presetSwapEnabled = 0;
    const presetSwapMs = 5000;

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
      if (syncUi) {
        setSliderValue(
          "lightPreset",
          index,
          (v) => lightPresets[Math.round(v)]?.label ?? `${v}`,
        );
      }
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
      if (syncUi) {
        setSliderValue(
          "motionPreset",
          index,
          (v) => motionPresets[Math.round(v)]?.label ?? `${v}`,
        );
      }
    };
    const applySparkPreset = (index: number, syncUi = false) => {
      const preset = sparkPresets[index];
      sparkConfig.intervalSeconds = preset.intervalSeconds;
      sparkConfig.pauseMs = preset.pauseMs;
      sparkConfig.drainRatePerMs = preset.drainRatePerMs;
      viz.setSparkIntervalSeconds(preset.intervalSeconds);
      viz.setSparkPauseMs(preset.pauseMs);
      viz.setSparkDrainRatePerMs(preset.drainRatePerMs);
      if (syncUi) {
        setSliderValue(
          "sparkPreset",
          index,
          (v) => sparkPresets[Math.round(v)]?.label ?? `${v}`,
        );
      }
    };

    applyLightPreset(lightPresetIndex);
    applyMotionPreset(motionPresetIndex);
    applySparkPreset(sparkPresetIndex);

    addSlider(
      "Light Preset",
      0,
      lightPresets.length - 1,
      1,
      lightPresetIndex,
      (v) => {
        lightPresetIndex = Math.round(v);
        applyLightPreset(lightPresetIndex);
      },
      (v) => lightPresets[Math.round(v)]?.label ?? `${v}`,
      "lightPreset",
    );
    addSlider(
      "Motion Preset",
      0,
      motionPresets.length - 1,
      1,
      motionPresetIndex,
      (v) => {
        motionPresetIndex = Math.round(v);
        applyMotionPreset(motionPresetIndex);
      },
      (v) => motionPresets[Math.round(v)]?.label ?? `${v}`,
      "motionPreset",
    );
    addSlider(
      "Spark Preset",
      0,
      sparkPresets.length - 1,
      1,
      sparkPresetIndex,
      (v) => {
        sparkPresetIndex = Math.round(v);
        applySparkPreset(sparkPresetIndex);
      },
      (v) => sparkPresets[Math.round(v)]?.label ?? `${v}`,
      "sparkPreset",
    );
    addSlider(
      "Preset Swap",
      0,
      1,
      1,
      presetSwapEnabled,
      (v) => {
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
      },
      (v) => (Math.round(v) === 1 ? "On" : "Off"),
    );
    addHeader("Seed");
    addSlider("Seed", 1, 9999, 1, lightConfig.seed, (v) => {
      const seed = Math.round(v);
      lightConfig.seed = seed;
      viz.setSeed(seed);
    });
    addSlider("Dwell (s)", 2, 15, 1, lightConfig.dwell, (v) => {
      lightConfig.dwell = v;
      viz.setDwellSeconds(v);
    });
    addSlider("Wander", 2, 16, 1, lightConfig.wander, (v) => {
      lightConfig.wander = v;
      viz.setWanderDistance(v);
    });
  }

  return {
    dispose: () => {
      toggle.remove();
      if (presetSwapTimer !== null) {
        window.clearInterval(presetSwapTimer);
        presetSwapTimer = null;
      }
    },
  };
}
