type MathConfigHost = {
  configPanel?: HTMLDivElement;
  configToggle?: HTMLButtonElement;
  setPanelOpen?: (open: boolean) => void;
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
};

export function setupMathConfig(viz: MathConfigHost) {
  const panel = document.getElementById(
    "math-config-panel",
  ) as HTMLDivElement | null;
  const toggle = document.getElementById(
    "math-config-toggle",
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
    const sliderId = `math-slider-${label.replace(/\s+/g, "-").toLowerCase()}`;
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
  addSlider("Radius", viz.cameraOrbitRadius, 8, 35, 0.5, (v) => {
    viz.cameraOrbitRadius = v;
  });
  addSlider("Height", viz.cameraHeight, -5, 15, 0.5, (v) => {
    viz.cameraHeight = v;
  });
  addSlider("Height Osc", viz.cameraHeightOsc, 0, 6, 0.1, (v) => {
    viz.cameraHeightOsc = v;
  });
  addSlider("Height Speed", viz.cameraHeightSpeed, 0, 2, 0.05, (v) => {
    viz.cameraHeightSpeed = v;
  });
  addSlider("Roll", viz.cameraRoll, -0.6, 0.6, 0.01, (v) => {
    viz.cameraRoll = v;
  });
  addSlider("Roll Speed", viz.cameraRollSpeed, 0, 1, 0.01, (v) => {
    viz.cameraRollSpeed = v;
  });
  addSlider(
    "Orbit Speed",
    viz.cameraOrbitSpeed,
    0,
    0.02,
    0.0005,
    (v) => {
      viz.cameraOrbitSpeed = v;
    },
    (v) => v.toFixed(4),
  );
  addSlider("Look Y", viz.cameraLookAtY, -5, 5, 0.5, (v) => {
    viz.cameraLookAtY = v;
  });

  addSection("Curve Shape");
  addSlider("Curve A", viz.curveA, -3, 3, 0.05, (v) => (viz.curveA = v));
  addSlider("Curve B", viz.curveB, -3, 3, 0.05, (v) => (viz.curveB = v));
  addSlider("Curve C", viz.curveC, -3, 3, 0.05, (v) => (viz.curveC = v));
  addSlider("Curve D", viz.curveD, -3, 3, 0.05, (v) => (viz.curveD = v));
  addSlider("Curve E", viz.curveE, -3, 3, 0.05, (v) => (viz.curveE = v));
  addSlider("Curve F", viz.curveF, -3, 3, 0.05, (v) => (viz.curveF = v));

  addSection("Grid");
  addSlider("Opacity", viz.gridOpacity, 0, 1, 0.05, (v) => {
    viz.gridOpacity = v;
  });
  addSlider("Oscillation", viz.gridOpacityOsc, 0, 0.7, 0.01, (v) => {
    viz.gridOpacityOsc = v;
  });
  addSlider("Osc Speed", viz.gridOscSpeed, 0, 2, 0.05, (v) => {
    viz.gridOscSpeed = v;
  });

  addSection("Orbits");
  addSlider(
    "Inner Speed",
    viz.innerOrbitSpeed,
    0,
    0.01,
    0.0005,
    (v) => {
      viz.innerOrbitSpeed = v;
    },
    (v) => v.toFixed(4),
  );
  addSlider(
    "Middle Speed",
    viz.middleOrbitSpeed,
    0,
    0.01,
    0.0005,
    (v) => {
      viz.middleOrbitSpeed = v;
    },
    (v) => v.toFixed(4),
  );
  addSlider(
    "Outer Speed",
    viz.outerOrbitSpeed,
    0,
    0.01,
    0.0005,
    (v) => {
      viz.outerOrbitSpeed = v;
    },
    (v) => v.toFixed(4),
  );

  addCopyButton();
}
