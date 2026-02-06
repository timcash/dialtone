type WebgpuTemplateConfigOptions = {
  spinSpeed: number;
  onSpinChange: (value: number) => void;
};

export function setupWebgpuTemplateConfig(options: WebgpuTemplateConfigOptions) {
  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "webgpu-template-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById(
    "webgpu-template-config-panel",
  ) as HTMLDivElement | null;
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

    const row = document.createElement("div");
    row.className = "earth-config-row";
    const label = document.createElement("label");
    label.className = "earth-config-label";
    label.htmlFor = "webgpu-template-spin";
    label.textContent = "Spin";
    const slider = document.createElement("input");
    slider.id = "webgpu-template-spin";
    slider.type = "range";
    slider.min = "0";
    slider.max = "2";
    slider.step = "0.01";
    slider.value = `${options.spinSpeed}`;
    row.appendChild(label);
    row.appendChild(slider);
    const valueEl = document.createElement("span");
    valueEl.className = "earth-config-value";
    valueEl.textContent = options.spinSpeed.toFixed(2);
    row.appendChild(valueEl);
    panel.appendChild(row);

    slider.addEventListener("input", () => {
      const value = parseFloat(slider.value);
      options.onSpinChange(value);
      valueEl.textContent = value.toFixed(2);
    });
  }

  return {
    dispose: () => {
      toggle.remove();
    },
  };
}
