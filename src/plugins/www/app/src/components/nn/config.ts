type NnConfigHost = any;

export function setupNnConfig(viz: NnConfigHost) {
  const panel = document.getElementById(
    "nn-config-panel",
  ) as HTMLDivElement | null;
  const toggle = document.getElementById(
    "nn-config-toggle",
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

  const addSection = (title: string) => {
    const header = document.createElement("h3");
    header.textContent = title;
    panel.appendChild(header);
  };

  const addSlider = (
    label: string,
    value: number,
    min: number,
    max: number,
    step: number,
    onInput: (v: number) => void,
    format: (v: number) => string = (v) => v.toFixed(2),
  ) => {
    const row = document.createElement("div");
    row.className = "earth-config-row";

    const labelWrap = document.createElement("label");
    const sliderId = `nn-slider-${label.replace(/\s+/g, "-").toLowerCase()}`;
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
  };

  const addCopyButton = () => {
    const button = document.createElement("button");
    button.type = "button";
    button.textContent = "Copy Config";
    button.addEventListener("click", () => {
      const payload = JSON.stringify(viz.buildConfigSnapshot(), null, 2);
      if (navigator.clipboard?.writeText) {
        navigator.clipboard.writeText(payload).catch(() => console.log(payload));
      } else {
        console.log(payload);
      }
    });
    panel.appendChild(button);
  };

  addSection("Camera");
  addSlider("Radius", viz.cameraRadius, 5, 30, 0.5, (v) => {
    viz.cameraRadius = v;
  });
  addSlider("Height", viz.cameraHeight, -5, 10, 0.5, (v) => {
    viz.cameraHeight = v;
  });
  addSlider("Height Osc", viz.cameraHeightOsc, 0, 5, 0.1, (v) => {
    viz.cameraHeightOsc = v;
  });
  addSlider("Height Speed", viz.cameraHeightSpeed, 0, 0.5, 0.01, (v) => {
    viz.cameraHeightSpeed = v;
  });
  addSlider(
    "Orbit Speed",
    viz.cameraOrbitSpeed,
    0,
    0.2,
    0.005,
    (v) => {
      viz.cameraOrbitSpeed = v;
    },
    (v) => v.toFixed(3),
  );

  addCopyButton();
}
