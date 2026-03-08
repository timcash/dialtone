import { setupApp } from "../../../../ui/src_v1/ui/ui";
import {
  getUISharedShellOverlays,
  registerUISharedSections,
  type UISharedSectionEntry,
  renderUISharedShell,
} from "../../../../ui/src_v1/ui/templates";
import { mountSignalTerminal, type SignalTerminal } from "./xterm";
import { mountSphereScene } from "./three_scene";
import {
  sendRoverCommand,
  startMockConnection,
  subscribeRoverState,
  type RoverState,
} from "./mock_connection";
import "../../../../ui/src_v1/ui/style.css";
import "./style.css";

const sectionEntries: UISharedSectionEntry[] = [
  { sectionID: "test-home-docs", template: "docs", title: "Overview" },
  { sectionID: "test-robot-docs", template: "docs", title: "Docs" },
  { sectionID: "test-telemetry-table", template: "table", title: "Telemetry" },
  { sectionID: "test-steering-table", template: "table", title: "Steering" },
  { sectionID: "test-key-params-table", template: "table", title: "Key Params" },
  { sectionID: "test-signals-terminal", template: "terminal", title: "Signals" },
  { sectionID: "test-camera-video", template: "camera", title: "Camera" },
  { sectionID: "test-settings-docs", template: "docs", title: "Settings" },
];

const { sections, menu } = setupApp({
  title: "dialtone.test",
  debug: true,
});

registerUISharedSections({
  sections,
  menu,
  entries: sectionEntries,
  decorate: (entry, container) => {
    decorateSection(entry, container);
  },
});

void sections.navigateTo("test-home-docs");

let signalTerminal: SignalTerminal | null = null;
let threeChatTerminal: SignalTerminal | null = null;
let roverState: RoverState = {
  connected: false,
  mode: "BOOT",
  batteryV: "0.0",
  speedMS: "0.0",
  altitudeM: "0.0",
  headingDeg: "0",
  latitude: "0.0000",
  longitude: "0.0000",
  satellites: "0",
  link: "offline",
  fps: "0",
  bitrate: "0.0 Mbps",
  feed: "mock-a",
  latencyMS: "0",
  logs: ["[mock] waiting for rover stream"],
  steeringProfile: [],
  keyParams: [],
};

function ctl() {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  };
}

function setLegendValue(section: ParentNode, label: string, value: string): void {
  const rows = Array.from(section.querySelectorAll(".shell-legend-telemetry div"));
  for (const row of rows) {
    const dt = row.querySelector("dt");
    const dd = row.querySelector("dd");
    if (dt?.textContent?.trim().toLowerCase() === label.toLowerCase() && dd) {
      dd.textContent = value;
    }
  }
}

function renderSignals(lines = roverState.logs): void {
  if (signalTerminal) {
    signalTerminal.setLines(lines);
  }
  if (threeChatTerminal) {
    threeChatTerminal.setLines(lines.slice(-18));
  }
}

function updateTableRows(sectionId: string, rows: Array<[string, string, string, string]>): void {
  const section = document.getElementById(sectionId);
  const tbody = section?.querySelector("tbody");
  if (!tbody) return;
  tbody.innerHTML = rows
    .map(
      ([key, value, status, detail]) =>
        `<tr><td>${key}</td><td>${value}</td><td>${status}</td><td>${detail}</td></tr>`,
    )
    .join("");
}

function labelForm(
  form: HTMLFormElement,
  labels: string[],
  inputAria: string,
  inputPlaceholder: string,
  submitAria: string,
): void {
  const buttons = Array.from(form.querySelectorAll("button"));
  buttons.slice(0, 9).forEach((button, index) => {
    button.textContent = labels[index];
    button.setAttribute("aria-label", labels[index]);
  });
  const input = form.querySelector("input");
  if (input) {
    input.setAttribute("aria-label", inputAria);
    input.setAttribute("placeholder", inputPlaceholder);
  }
  const submit = buttons[9];
  if (submit) {
    submit.textContent = submitAria.replace(/^.*\s/, "");
    submit.setAttribute("aria-label", submitAria);
  }
}

function bindCommandForm(
  section: HTMLElement,
  labels: string[],
  commandPrefix: string,
  inputAria: string,
  submitAria: string,
): void {
  if (section.dataset.formBound === "1") return;
  const form = section.querySelector("form");
  if (!(form instanceof HTMLFormElement)) return;
  section.dataset.formBound = "1";
  labelForm(form, labels, inputAria, `${commandPrefix} command`, submitAria);
  const buttons = Array.from(form.querySelectorAll("button"));
  buttons.slice(0, 8).forEach((button, index) => {
    button.addEventListener("click", () => {
      sendRoverCommand(`${commandPrefix}.${labels[index].toLowerCase().replace(/\s+/g, "_")}`);
      console.log(`fixture:button:${commandPrefix}:${labels[index]}`);
    });
  });
  const modeButton = buttons[8];
  modeButton?.addEventListener("click", () => {
    sendRoverCommand(`${commandPrefix}.mode`);
    console.log(`fixture:mode:${commandPrefix}`);
  });
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const input = form.querySelector<HTMLInputElement>("input");
    sendRoverCommand(`${commandPrefix}.submit`, { value: input?.value ?? "" });
    console.log(`fixture:submit:${commandPrefix}:${input?.value ?? ""}`);
  });
}

function updateOverview(): void {
  const section = document.getElementById("test-home-docs");
  if (!section) return;
  const subtitle = section.querySelector(".shell-legend-text p");
  if (subtitle) {
    subtitle.textContent = `Shared template harness for Robot-style UIs. Link=${roverState.link} mode=${roverState.mode} battery=${roverState.batteryV}V`;
  }
  const article = section.querySelector("article");
  if (article) {
    article.innerHTML = `
      <h2>One UI, two backends</h2>
      <p>The browser client always talks to the Robot contract: <code>/api/init</code>, <code>/natsws</code>, and <code>/stream</code>.</p>
      <p>Swap between the real rover and the local mock server by changing the Vite proxy target. The UI and browser tests stay unchanged.</p>
      <pre>TEST_UI_BACKEND_ORIGIN=http://127.0.0.1:8787 npm run dev
TEST_UI_BACKEND_ORIGIN=https://rover-1.dialtone.earth npm run dev</pre>
    `;
  }
}

function updateTelemetrySections(): void {
  updateTableRows("test-telemetry-table", [
    ["mode", roverState.mode, "live", `${roverState.link} link`],
    ["battery", `${roverState.batteryV} V`, "live", "mavlink.sys_status"],
    ["speed", `${roverState.speedMS} m/s`, "live", "mavlink.vfr_hud"],
    ["altitude", `${roverState.altitudeM} m`, "live", "mavlink.global_position_int"],
    ["heading", `${roverState.headingDeg} deg`, "live", "mavlink.vfr_hud"],
    ["gps", `${roverState.latitude}, ${roverState.longitude}`, "live", `${roverState.satellites} sats`],
  ]);
  const section = document.getElementById("test-telemetry-table");
  if (!section) return;
  setLegendValue(section, "source", roverState.connected ? "real-or-mock" : "offline");
  setLegendValue(section, "status", roverState.link);
  setLegendValue(section, "rows", "6");
  setLegendValue(section, "rate", `${roverState.fps || "0"}hz`);
  setLegendValue(section, "view", roverState.mode);
  setLegendValue(section, "errors", roverState.connected ? "0" : "1");
  setLegendValue(section, "sort", "live");
  setLegendValue(section, "filter", "none");
}

function updateSteeringSection(): void {
  updateTableRows(
    "test-steering-table",
    roverState.steeringProfile.map(([key, value, status]) => [key, value, status, "rover.steering"]),
  );
  const section = document.getElementById("test-steering-table");
  if (!section) return;
  setLegendValue(section, "source", "steering");
  setLegendValue(section, "status", roverState.link);
  setLegendValue(section, "rows", String(roverState.steeringProfile.length));
  setLegendValue(section, "rate", "2hz");
  setLegendValue(section, "view", "edit");
  setLegendValue(section, "errors", "0");
  setLegendValue(section, "sort", "manual");
  setLegendValue(section, "filter", roverState.mode);
}

function updateKeyParamsSection(): void {
  updateTableRows(
    "test-key-params-table",
    roverState.keyParams.map(([key, value, status]) => [key, value, status, "rover.params"]),
  );
  const section = document.getElementById("test-key-params-table");
  if (!section) return;
  setLegendValue(section, "source", "params");
  setLegendValue(section, "status", roverState.link);
  setLegendValue(section, "rows", String(roverState.keyParams.length));
  setLegendValue(section, "rate", "1hz");
  setLegendValue(section, "view", "key");
  setLegendValue(section, "errors", "0");
  setLegendValue(section, "sort", "name");
  setLegendValue(section, "filter", roverState.mode);
}

function updateThreeSections(): void {
  const overview = document.getElementById("test-three-overview-stage");
  const calc = document.getElementById("test-three-calculator-stage");
  const subtitle = overview?.querySelector(".shell-legend-text p");
  if (subtitle) {
    subtitle.textContent = `Live rover state over a minimal three.js overview. Mode=${roverState.mode} speed=${roverState.speedMS}m/s`;
  }
  if (calc) {
    setLegendValue(calc, "scene", roverState.feed);
    setLegendValue(calc, "camera", "orbit");
    setLegendValue(calc, "fps", roverState.fps);
    setLegendValue(calc, "nodes", "22");
    setLegendValue(calc, "edges", "43");
    setLegendValue(calc, "labels", "on");
    setLegendValue(calc, "gpu", "on");
    setLegendValue(calc, "mode", roverState.mode);
  }
}

function updateCameraSection(): void {
  const section = document.getElementById("test-camera-video");
  if (!section) return;
  const img = section.querySelector<HTMLImageElement>("img.camera-stage");
  if (img && !img.src.includes("/stream")) {
    img.src = `/stream?t=${Date.now()}`;
  }
  setLegendValue(section, "stream", roverState.feed);
  setLegendValue(section, "codec", "mjpeg");
  setLegendValue(section, "fps", roverState.fps);
  setLegendValue(section, "latency", `${roverState.latencyMS}ms`);
  setLegendValue(section, "drops", roverState.connected ? "0" : "1");
  setLegendValue(section, "bitrate", roverState.bitrate);
  setLegendValue(section, "res", "1280x720");
  setLegendValue(section, "audio", "off");
}

function updateSettingsDocs(): void {
  const section = document.getElementById("test-settings-docs");
  const article = section?.querySelector("article");
  if (article) {
    article.innerHTML = `
      <h2>Backend toggle</h2>
      <p><strong>Current link:</strong> ${roverState.link}</p>
      <p><strong>Current mode:</strong> ${roverState.mode}</p>
      <p>For local mock mode, start the Go server below and point Vite at it:</p>
      <pre>go run ./src/plugins/test/src_v1/mock_server --listen :8787
TEST_UI_BACKEND_ORIGIN=http://127.0.0.1:8787 npm run dev</pre>
      <p>For the real rover, point Vite at the running Robot service instead.</p>
    `;
  }
}

function refreshLiveState(): void {
  updateOverview();
  updateTelemetrySections();
  updateSteeringSection();
  updateKeyParamsSection();
  updateThreeSections();
  updateCameraSection();
  updateSettingsDocs();
  renderSignals(roverState.logs);
}

function decorateSection(entry: UISharedSectionEntry, container: HTMLElement): void {
  if (container.dataset.bound === "1") return;
  container.dataset.bound = "1";

  if (entry.sectionID === "test-home-docs") {
    const title = container.querySelector(".shell-legend-text h1");
    if (title) title.textContent = "Robot template harness";
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-robot-docs") {
    const title = container.querySelector(".shell-legend-text h1");
    const subtitle = container.querySelector(".shell-legend-text p");
    const article = container.querySelector("article");
    if (title) title.textContent = "Robot docs template";
    if (subtitle) subtitle.textContent = "Docs-style section matching the Robot page shell without carrying Robot-specific component code.";
    if (article) {
      article.innerHTML = `
        <h2>Reduced template map</h2>
        <ul>
          <li><code>hero</code> -> <code>three</code> fullscreen with text legend</li>
          <li><code>docs</code> -> <code>docs</code></li>
          <li><code>table</code>, <code>steering</code>, <code>key params</code> -> <code>table</code></li>
          <li><code>three</code> -> <code>three</code> calculator with telemetry legend and optional chatlog</li>
          <li><code>xterm</code> -> <code>terminal</code></li>
          <li><code>video</code> -> <code>camera</code></li>
          <li><code>settings</code> -> <code>docs</code> or a future small settings shell</li>
        </ul>
      `;
    }
    return;
  }

  if (entry.sectionID === "test-three-overview-stage") {
    renderUISharedShell(container, {
      underlay: "three",
      mode: "fullscreen",
      legend: "text",
      form: false,
      chatlog: false,
    });
    const title = container.querySelector(".shell-legend-text h1");
    const subtitle = container.querySelector(".shell-legend-text p");
    if (title) title.textContent = "Robot three overview";
    if (subtitle) {
      subtitle.textContent =
        "Fullscreen three.js overview using the reduced shared shell. This is the source-of-truth hero pattern for plugins that need a live scene without the calculator form.";
    }
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-telemetry-table") {
    bindCommandForm(
      container,
      ["Refresh", "Clear", "Signals", "Focus", "Mark", "Trace", "Diff", "Tail", "Browse"],
      "telemetry",
      "Telemetry Query Input",
      "Telemetry Submit",
    );
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-steering-table") {
    bindCommandForm(
      container,
      ["Prev", "Next", "-100", "-10", "+10", "+100", "Save", "Reset", "Edit"],
      "steering",
      "Steering Settings Input",
      "Steering Submit",
    );
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-key-params-table") {
    bindCommandForm(
      container,
      ["Refresh", "Search", "Export", "Pin", "Reveal", "Audit", "Diff", "Tail", "Inspect"],
      "params",
      "Key Params Input",
      "Key Params Submit",
    );
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-signals-terminal") {
    const legend = container.querySelector(".shell-legend-telemetry dl");
    if (legend) {
      legend.innerHTML = `
        <div><dt>stream</dt><dd>logs.test</dd></div>
        <div><dt>status</dt><dd>live</dd></div>
        <div><dt>level</dt><dd>info</dd></div>
        <div><dt>tail</dt><dd>on</dd></div>
        <div><dt>errors</dt><dd>0</dd></div>
        <div><dt>warn</dt><dd>1</dd></div>
        <div><dt>info</dt><dd>${roverState.logs.length}</dd></div>
        <div><dt>mode</dt><dd>cursor</dd></div>
      `;
    }
    const terminal = container.querySelector<HTMLElement>(".xterm-primary");
    if (terminal) {
      terminal.setAttribute("aria-label", "Fixture Log");
      signalTerminal = mountSignalTerminal(terminal);
    }
    bindCommandForm(
      container,
      ["Left", "Right", "Up", "Down", "Home", "End", "Select", "Copy", "Cursor"],
      "terminal",
      "Log Command Input",
      "Log Submit",
    );
    renderSignals(roverState.logs);
    return;
  }

  if (entry.sectionID === "test-camera-video") {
    bindCommandForm(
      container,
      ["Feed A", "Feed B", "Wide", "Zoom", "IR", "Map", "Log", "Bookmark", "View"],
      "camera",
      "Camera Input",
      "Camera Submit",
    );
    refreshLiveState();
    return;
  }

  if (entry.sectionID === "test-settings-docs") {
    const title = container.querySelector(".shell-legend-text h1");
    const subtitle = container.querySelector(".shell-legend-text p");
    if (title) title.textContent = "Robot settings template";
    if (subtitle) subtitle.textContent = "Reduced docs/settings page for backend selection and test instructions.";
    refreshLiveState();
  }
}

sections.register("test-three-overview-stage", {
  containerId: "test-three-overview-stage",
  canonicalName: "test-three-overview-stage",
  load: async () => {
    const container = document.getElementById("test-three-overview-stage");
    if (!container) throw new Error("test-three-overview-stage container not found");
    decorateSection({ sectionID: "test-three-overview-stage", template: "docs", title: "Three" }, container);
    const canvas = container.querySelector("canvas");
    if (canvas instanceof HTMLCanvasElement) {
      return mountSphereScene(canvas);
    }
    return ctl();
  },
  overlays: getUISharedShellOverlays({
    underlay: "three",
    mode: "fullscreen",
    legend: "text",
    form: false,
  }),
});
menu.addButton("Three", "Open Three", () => {
  void sections.navigateTo("test-three-overview-stage");
});

sections.register("test-three-calculator-stage", {
  containerId: "test-three-calculator-stage",
  canonicalName: "test-three-calculator-stage",
  load: async () => {
    const container = document.getElementById("test-three-calculator-stage");
    if (!container) throw new Error("test-three-calculator-stage container not found");
    renderUISharedShell(container, {
      underlay: "three",
      mode: "calculator",
      legend: "telemetry",
      form: true,
      chatlog: true,
    });
    const chatlogHost = container.querySelector<HTMLElement>(".shell-chatlog-terminal");
    if (chatlogHost) {
      threeChatTerminal = mountSignalTerminal(chatlogHost);
      const chatlog = container.querySelector<HTMLElement>(".shell-chatlog");
      if (chatlog) {
        chatlog.hidden = false;
      }
    }
    const form = container.querySelector("form");
    if (form instanceof HTMLFormElement) {
      labelForm(
        form,
        ["Back", "Add", "Link", "Clear", "Open", "Rename", "Focus", "Labels On", "Graph"],
        "Three Input",
        "select node to rename",
        "Three Submit",
      );
      bindCommandForm(container, ["Back", "Add", "Link", "Clear", "Open", "Rename", "Focus", "Labels On", "Graph"], "three", "Three Input", "Three Submit");
    }
    refreshLiveState();
    const canvas = container.querySelector("canvas");
    if (canvas instanceof HTMLCanvasElement) {
      return mountSphereScene(canvas);
    }
    return ctl();
  },
  overlays: getUISharedShellOverlays({
    underlay: "three",
    mode: "calculator",
    legend: "telemetry",
    form: true,
    chatlog: true,
  }),
});
menu.addButton("Three Calc", "Open Three Calc", () => {
  void sections.navigateTo("test-three-calculator-stage");
});

subscribeRoverState((next) => {
  roverState = next;
  refreshLiveState();
});

void startMockConnection();
renderSignals(roverState.logs);
console.log("fixture:ready");
