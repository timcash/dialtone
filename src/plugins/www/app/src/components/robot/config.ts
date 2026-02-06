type RobotConfigHost = any;

export function setupRobotConfig(viz: RobotConfigHost) {
  const panel = document.getElementById(
    "robot-config-panel",
  ) as HTMLDivElement | null;
  const toggle = document.getElementById(
    "robot-config-toggle",
  ) as HTMLButtonElement | null;
  if (!panel || !toggle) return;

  viz.configPanel = panel;
  viz.configToggle = toggle;

  const setOpen = (open: boolean) => {
    panel.hidden = !open;
    panel.style.display = open ? "grid" : "none";
    toggle.setAttribute("aria-expanded", String(open));
  };
  viz.setPanelOpen = setOpen;

  setOpen(false);
  toggle.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    setOpen(panel.hidden);
  });

  const addHeader = (text: string) => {
    const header = document.createElement("h3");
    header.textContent = text;
    panel.appendChild(header);
  };

  const addSlider = (
    label: string,
    value: number,
    min: number,
    max: number,
    step: number,
    onInput: (v: number) => void,
    format: (v: number) => string = (v) => `${Math.round(v)}°`,
  ) => {
    const row = document.createElement("div");
    row.className = "earth-config-row";

    const labelWrap = document.createElement("label");
    const sliderId = `robot-slider-${label.replace(/\s+/g, "-").toLowerCase()}`;
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

    return { slider, valueEl };
  };

  const addCheckbox = (
    label: string,
    checked: boolean,
    onChange: (v: boolean) => void,
  ) => {
    const row = document.createElement("div");
    row.className = "earth-config-row";

    const labelWrap = document.createElement("label");
    labelWrap.style.display = "flex";
    labelWrap.style.alignItems = "center";
    labelWrap.style.gap = "8px";

    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    checkbox.checked = checked;

    const text = document.createElement("span");
    text.textContent = label;

    labelWrap.appendChild(checkbox);
    labelWrap.appendChild(text);
    row.appendChild(labelWrap);
    panel.appendChild(row);

    checkbox.addEventListener("change", () => onChange(checkbox.checked));
  };

  const addButton = (label: string, onClick: () => void) => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = label;
    button.addEventListener("click", onClick);
    panel.appendChild(button);
  };

  addHeader("IK Mode");
  addCheckbox("Auto Track Target", viz.autoAnimate, (v) => {
    viz.autoAnimate = v;
  });
  addButton("New Target", () => viz.pickNewTarget());

  addHeader("Camera");
  addSlider("Distance", viz.cameraDistance, 4, 20, 0.5, (v) => {
    viz.cameraDistance = v;
  }, (v) => v.toFixed(1));
  addSlider("Height", viz.cameraHeight, -2, 8, 0.25, (v) => {
    viz.cameraHeight = v;
  }, (v) => v.toFixed(2));
  addSlider("Orbit", viz.cameraOrbitSpeed, 0, 0.02, 0.0005, (v) => {
    viz.cameraOrbitSpeed = v;
  }, (v) => v.toFixed(4));

  addHeader("Target");
  addSlider("Target X", viz.targetPosition.x, -4, 4, 0.1, (v) => {
    viz.targetPosition.x = v;
    viz.updateTargetLine();
  }, (v) => v.toFixed(1));
  addSlider("Target Y", viz.targetPosition.y, -2, 6, 0.1, (v) => {
    viz.targetPosition.y = v;
    viz.updateTargetLine();
  }, (v) => v.toFixed(1));
  addSlider("Target Z", viz.targetPosition.z, -4, 4, 0.1, (v) => {
    viz.targetPosition.z = v;
    viz.updateTargetLine();
  }, (v) => v.toFixed(1));

  addHeader("Joint Angles");
  const jointConfigs = [
    { name: "Base (Y)", min: -180, max: 180, initial: 0 },
    { name: "Shoulder (Z)", min: -100, max: 100, initial: 30 },
    { name: "Elbow (Y)", min: -180, max: 180, initial: 0 },
    { name: "Forearm (Z)", min: -100, max: 100, initial: -45 },
    { name: "Wrist (Z)", min: -100, max: 100, initial: -20 },
  ];

  jointConfigs.forEach((config, i) => {
    const { slider, valueEl } = addSlider(
      config.name,
      config.initial,
      config.min,
      config.max,
      1,
      (v) => {
        viz.robotArm.joints[i].setAngle(v);
        viz.autoAnimate = false;
      },
    );
    viz.sliders.push({ slider, valueEl });
  });

  addHeader("Gripper");
  addSlider(
    "Grip",
    0.5,
    0,
    1,
    0.01,
    (v) => viz.robotArm.setGrip(v),
    (v) => `${Math.round(v * 100)}%`,
  );
}
