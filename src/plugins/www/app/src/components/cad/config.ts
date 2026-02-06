import type { CADViewer } from "./index";

export function setupCadConfig(viewer: CADViewer) {
  viewer.panelEl = document.getElementById(
    "cad-config-panel",
  ) as HTMLDivElement | null;
  viewer.toggleEl = document.getElementById(
    "cad-config-toggle",
  ) as HTMLButtonElement | null;
  if (!viewer.panelEl || !viewer.toggleEl) return;

  viewer.setPanelOpen(false);
  viewer.toggleEl.addEventListener("click", (e) => {
    e.preventDefault();
    e.stopPropagation();
    viewer.setPanelOpen(viewer.panelEl!.hidden);
  });

  const addHeader = (text: string) => {
    const header = document.createElement("h3");
    header.textContent = text;
    viewer.panelEl?.appendChild(header);
  };

  const addSlider = (
    id: string,
    label: string,
    min: number,
    max: number,
    step: number,
  ) => {
    const row = document.createElement("div");
    row.className = "earth-config-row cad-config-row";
    const labelWrap = document.createElement("label");
    const sliderId = `cad-slider-${id}`;
    labelWrap.className = "earth-config-label";
    labelWrap.htmlFor = sliderId;
    labelWrap.textContent = label;
    const slider = document.createElement("input");
    slider.type = "range";
    slider.id = sliderId;
    slider.min = `${min}`;
    slider.max = `${max}`;
    slider.step = `${step}`;
    // @ts-ignore
    slider.value = String(viewer.params[id]);
    row.appendChild(labelWrap);
    row.appendChild(slider);
    const valueEl = document.createElement("span");
    valueEl.className = "earth-config-value";
    valueEl.textContent = slider.value;
    row.appendChild(valueEl);
    viewer.panelEl?.appendChild(row);

    slider.addEventListener("input", () => {
      const v = parseFloat(slider.value);
      // @ts-ignore
      viewer.params[id] = v;
      valueEl.textContent = slider.value;
      viewer.debouncedUpdate();
    });
  };

  addHeader("Gear Parameters");

  viewer.offlineWarningEl = document.createElement("div");
  viewer.offlineWarningEl.className = "offline-warning";
  viewer.offlineWarningEl.innerHTML =
    "⚠️ CAD Server Offline. Start with <code>./dialtone.sh www cad demo</code> to enable parametric changes.";
  viewer.offlineWarningEl.hidden = true;
  viewer.panelEl?.appendChild(viewer.offlineWarningEl);

  addSlider("outer_diameter", "Outer Dia", 20, 200, 1);
  addSlider("inner_diameter", "Inner Dia", 5, 100, 1);
  addSlider("thickness", "Thickness", 2, 50, 1);
  addSlider("num_teeth", "Num Teeth", 5, 100, 1);
  addSlider("num_mounting_holes", "Mount Holes", 0, 12, 1);
  addSlider("mounting_hole_diameter", "Hole Dia", 2, 20, 1);

  const dlBtn = document.createElement("button");
  dlBtn.className = "premium-button";
  dlBtn.textContent = "Download STL";
  dlBtn.style.marginTop = "1rem";
  dlBtn.addEventListener("click", (e) => {
    e.preventDefault();
    viewer.downloadSTL();
  });
  viewer.panelEl?.appendChild(dlBtn);

  addHeader("Visualization");
  const addTranslationSlider = () => {
    const row = document.createElement("div");
    row.className = "earth-config-row cad-config-row";
    const labelWrap = document.createElement("label");
    const sliderId = "cad-slider-translation-x";
    labelWrap.className = "earth-config-label";
    labelWrap.htmlFor = sliderId;
    labelWrap.textContent = "Translation X";
    const slider = document.createElement("input");
    slider.type = "range";
    slider.id = sliderId;
    slider.min = "-200";
    slider.max = "200";
    slider.step = "1";
    slider.value = String(viewer.translationX);
    row.appendChild(labelWrap);
    row.appendChild(slider);
    const valueEl = document.createElement("span");
    valueEl.className = "earth-config-value";
    valueEl.textContent = slider.value;
    row.appendChild(valueEl);
    viewer.panelEl?.appendChild(row);

    slider.addEventListener("input", () => {
      viewer.translationX = parseFloat(slider.value);
      valueEl.textContent = slider.value;
      if (viewer.gearGroup) {
        viewer.gearGroup.position.x = viewer.translationX;
      }
    });
  };
  addTranslationSlider();

  const divider = document.createElement("div");
  divider.className = "code-divider";
  viewer.panelEl?.appendChild(divider);

  const ghBtn = document.createElement("button");
  ghBtn.className = "premium-button github-button";
  ghBtn.innerHTML = "<span>View Source on GitHub</span>";
  ghBtn.style.background = "rgba(255, 255, 255, 0.1)";
  ghBtn.style.border = "1px solid rgba(255, 255, 255, 0.2)";
  ghBtn.addEventListener("click", (e) => {
    e.preventDefault();
    window.open(
      "https://github.com/timcash/dialtone/blob/main/src/plugins/cad/backend/main.py",
      "_blank",
    );
  });
  viewer.panelEl?.appendChild(ghBtn);
}
