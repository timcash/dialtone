import { ProceduralOrbit } from "./index";

export function setupConfigPanel(orbit: ProceduralOrbit) {
  const panel = document.getElementById(
    "earth-config-panel",
  ) as HTMLDivElement | null;
  const toggle = document.getElementById(
    "earth-config-toggle",
  ) as HTMLButtonElement | null;
  if (!panel || !toggle) return { panel: null, toggle: null };

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

  const addSection = (title: string) => {
    const header = document.createElement("h3");
    header.textContent = title;
    panel.appendChild(header);
  };

  const addSlider = (
    key: string,
    label: string,
    value: number,
    min: number,
    max: number,
    step: number,
    onInput: (v: number) => void,
    format: (v: number) => string = (v) => v.toFixed(3),
    getValue?: () => number,
  ) => {
    const row = document.createElement("div");
    row.className = "earth-config-row";
    const labelWrap = document.createElement("label");
    const sliderId = `earth-config-${key}`;
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
    orbit.configValueMap.set(key, valueEl);
    orbit.configSliderMap.set(key, {
      slider,
      valueEl,
      format,
      getValue: getValue ?? (() => parseFloat(slider.value)),
    });
    slider.addEventListener("input", () => {
      const next = parseFloat(slider.value);
      onInput(next);
      valueEl.textContent = format(next);
    });
  };

  const addCopyButton = () => {
    const btn = document.createElement("button");
    btn.textContent = "Copy Config";
    btn.addEventListener("click", () => {
      const payload = JSON.stringify(orbit.buildConfigSnapshot(), null, 2);
      navigator.clipboard?.writeText(payload);
    });
    panel.appendChild(btn);
  };

  addSection("Rotation");
  const toPeriodSeconds = (rotSpeedRadPerSec: number) => {
    if (!isFinite(rotSpeedRadPerSec) || rotSpeedRadPerSec <= 0) return Infinity;
    return (Math.PI * 2) / rotSpeedRadPerSec;
  };
  const toRotSpeed = (periodSeconds: number) => {
    if (!isFinite(periodSeconds) || periodSeconds <= 0) return 0;
    return (Math.PI * 2) / periodSeconds;
  };
  addSlider(
    "earthRotPeriod",
    "Earth",
    Math.min(60, toPeriodSeconds(orbit.earthRotSpeed)),
    1,
    60,
    1,
    (v: number) => (orbit.earthRotSpeed = toRotSpeed(v)),
    (v: number) => (isFinite(v) ? v.toFixed(0) : "∞"),
    () => {
      const v = toPeriodSeconds(orbit.earthRotSpeed);
      return Math.min(60, Math.max(1, v));
    },
  );
  addSlider(
    "sunOrbitSpeed",
    "Sun Orbit",
    orbit.sunOrbitSpeed,
    0,
    0.005,
    0.0001,
    (v: number) => (orbit.sunOrbitSpeed = v),
    (v: number) => v.toFixed(4),
    () => orbit.sunOrbitSpeed,
  );
  addSlider(
    "sunOrbitPos",
    "Sun Pos",
    orbit.sunOrbitAngleRad,
    0,
    Math.PI * 2,
    0.01,
    (v: number) => orbit.setSunOrbitAngleRad(v),
    (v: number) => v.toFixed(2),
    () => orbit.sunOrbitAngleRad,
  );

  addSection("Atmosphere");
  addSlider(
    "cloudAmount",
    "Cloud Amt",
    orbit.cloudAmount,
    0,
    1,
    0.01,
    (v: number) => (orbit.cloudAmount = v),
    (v: number) => v.toFixed(2),
    () => orbit.cloudAmount,
  );
  addSlider(
    "cloudBrightness",
    "Brightness",
    orbit.cloudBrightness,
    0,
    5,
    0.1,
    (v: number) => (orbit.cloudBrightness = v),
    (v: number) => v.toFixed(1),
    () => orbit.cloudBrightness,
  );

  addSection("Cloud Layer 1");
  addSlider(
    "c1Speed",
    "Speed",
    orbit.cloud1RotSpeed * 100000,
    0,
    50,
    1,
    (v: number) => (orbit.cloud1RotSpeed = v / 100000),
    (v: number) => v.toFixed(0),
    () => orbit.cloud1RotSpeed * 100000,
  );
  addSlider(
    "c1Opacity",
    "Opacity",
    orbit.cloud1Opacity,
    0.5,
    1,
    0.01,
    (v: number) => (orbit.cloud1Opacity = v),
    (v: number) => v.toFixed(2),
    () => orbit.cloud1Opacity,
  );

  addSection("Cloud Layer 2");
  addSlider(
    "c2Speed",
    "Speed",
    orbit.cloud2RotSpeed * 100000,
    0,
    50,
    1,
    (v: number) => (orbit.cloud2RotSpeed = v / 100000),
    (v: number) => v.toFixed(0),
    () => orbit.cloud2RotSpeed * 100000,
  );
  addSlider(
    "c2Opacity",
    "Opacity",
    orbit.cloud2Opacity,
    0.5,
    1,
    0.01,
    (v: number) => (orbit.cloud2Opacity = v),
    (v: number) => v.toFixed(2),
    () => orbit.cloud2Opacity,
  );

  addSection("Camera");
  addSlider(
    "distance",
    "Distance",
    orbit.cameraDistance,
    0,
    30,
    0.5,
    (v: number) => (orbit.cameraDistance = v),
    (v: number) => v.toFixed(1),
    () => orbit.cameraDistance,
  );
  addSlider(
    "yaw",
    "Yaw",
    orbit.cameraYaw,
    0,
    Math.PI * 2,
    0.01,
    (v: number) => (orbit.cameraYaw = v),
    (v: number) => v.toFixed(2),
    () => orbit.cameraYaw,
  );
  addSlider(
    "orbit",
    "Orbit",
    orbit.cameraOrbit,
    0,
    Math.PI * 2,
    0.01,
    (v: number) => (orbit.cameraOrbit = v),
    (v: number) => v.toFixed(2),
    () => orbit.cameraOrbit,
  );

  addCopyButton();

  return {
    panel,
    toggle,
    setOpen,
  };
}

