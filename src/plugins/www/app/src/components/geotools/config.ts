type GeoToolsConfigOptions = {
  currentResolution: number;
  onResolutionChange: (value: number) => void;
  onFile: (file: File) => Promise<void> | void;
  onConvert: () => void;
  onDownload: () => void;
  getStatusText: () => string;
};

export function setupGeoToolsConfig(options: GeoToolsConfigOptions) {
  const controls = document.querySelector(".top-right-controls");
  const toggle = document.createElement("button");
  toggle.id = "geotools-config-toggle";
  toggle.className = "earth-config-toggle";
  toggle.type = "button";
  toggle.setAttribute("aria-expanded", "false");
  toggle.textContent = "Config";
  controls?.prepend(toggle);

  const panel = document.getElementById(
    "geotools-config-panel",
  ) as HTMLDivElement | null;
  let statusValue: HTMLSpanElement | null = null;
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

    const makeButton = (label: string) => {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "earth-config-toggle";
      button.textContent = label;
      return button;
    };

    const row = document.createElement("div");
    row.className = "earth-config-row about-config-row";
    const label = document.createElement("label");
    label.className = "earth-config-label";
    label.htmlFor = "geotools-file";
    label.textContent = "GeoJSON";
    const fileInput = document.createElement("input");
    fileInput.type = "file";
    fileInput.id = "geotools-file";
    fileInput.accept = ".json,.geojson,application/geo+json";
    row.appendChild(label);
    row.appendChild(fileInput);
    panel.appendChild(row);

    const resolutionRow = document.createElement("div");
    resolutionRow.className = "earth-config-row about-config-row";
    const resolutionLabel = document.createElement("label");
    const resolutionId = "geotools-resolution";
    resolutionLabel.className = "earth-config-label";
    resolutionLabel.htmlFor = resolutionId;
    resolutionLabel.textContent = "H3 Resolution";
    const resolutionInput = document.createElement("input");
    resolutionInput.type = "range";
    resolutionInput.id = resolutionId;
    resolutionInput.min = "0";
    resolutionInput.max = "5";
    resolutionInput.step = "1";
    resolutionInput.value = `${options.currentResolution}`;
    resolutionRow.appendChild(resolutionLabel);
    resolutionRow.appendChild(resolutionInput);
    const resolutionValue = document.createElement("span");
    resolutionValue.className = "earth-config-value";
    resolutionValue.textContent = `${options.currentResolution}`;
    resolutionRow.appendChild(resolutionValue);
    panel.appendChild(resolutionRow);

    const statusRow = document.createElement("div");
    statusRow.className = "earth-config-row about-config-row";
    const statusLabel = document.createElement("label");
    statusLabel.className = "earth-config-label";
    statusLabel.textContent = "Status";
    statusRow.appendChild(statusLabel);
    statusValue = document.createElement("span");
    statusValue.className = "earth-config-value";
    statusRow.appendChild(statusValue);
    panel.appendChild(statusRow);

    const buttonsRow = document.createElement("div");
    buttonsRow.className = "earth-config-row about-config-row";
    const buttonsLabel = document.createElement("label");
    buttonsLabel.className = "earth-config-label";
    buttonsLabel.textContent = "Actions";
    const convertButton = makeButton("Convert");
    const downloadButton = makeButton("Download H3");
    buttonsRow.appendChild(buttonsLabel);
    buttonsRow.appendChild(convertButton);
    buttonsRow.appendChild(downloadButton);
    panel.appendChild(buttonsRow);

    fileInput.addEventListener("change", async () => {
      const file = fileInput.files?.[0];
      if (!file) return;
      await options.onFile(file);
      updateStatus();
    });

    resolutionInput.addEventListener("input", () => {
      const value = parseInt(resolutionInput.value, 10);
      resolutionValue.textContent = `${value}`;
      options.onResolutionChange(value);
      updateStatus();
    });

    convertButton.addEventListener("click", () => {
      options.onConvert();
      updateStatus();
    });

    downloadButton.addEventListener("click", () => options.onDownload());
  }

  const updateStatus = () => {
    if (!statusValue) return;
    statusValue.textContent = options.getStatusText();
  };

  updateStatus();

  return {
    updateStatus,
    dispose: () => {
      toggle.remove();
    },
  };
}
