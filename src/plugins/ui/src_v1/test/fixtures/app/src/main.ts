import { setupApp } from "../../../../ui/ui";
import {
  getUISharedTemplate,
  registerUISharedSections,
  type UISharedTemplateID,
  type UISharedSectionEntry,
} from "../../../../ui/templates";
import * as THREE from "three";
import "../../../../ui/style.css";
import "./style.css";

function ctl(): {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
} {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  };
}

function mountSphereScene(canvas: HTMLCanvasElement): {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
} {
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: true,
  });
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
  renderer.setClearColor(0x000000, 1);

  const scene = new THREE.Scene();
  const camera = new THREE.PerspectiveCamera(50, 1, 0.1, 100);
  camera.position.set(0, 0.3, 3.2);

  const sphere = new THREE.Mesh(
    new THREE.SphereGeometry(0.95, 48, 32),
    new THREE.MeshStandardMaterial({
      color: 0x6fa8ff,
      roughness: 0.35,
      metalness: 0.08,
    }),
  );
  scene.add(sphere);

  const ground = new THREE.Mesh(
    new THREE.CircleGeometry(2.2, 48),
    new THREE.MeshStandardMaterial({
      color: 0x0d1522,
      roughness: 0.9,
      metalness: 0.02,
    }),
  );
  ground.rotation.x = -Math.PI / 2;
  ground.position.y = -1.12;
  scene.add(ground);

  scene.add(new THREE.AmbientLight(0xffffff, 0.45));
  const key = new THREE.DirectionalLight(0xffffff, 1.2);
  key.position.set(2.2, 2.8, 2.4);
  scene.add(key);

  const rim = new THREE.PointLight(0x89b8ff, 0.8, 12);
  rim.position.set(-2.2, 0.8, -1.8);
  scene.add(rim);

  const clock = new THREE.Clock();
  let raf = 0;
  let active = true;

  const resize = () => {
    const width = Math.max(
      1,
      canvas.clientWidth || canvas.parentElement?.clientWidth || 1,
    );
    const height = Math.max(
      1,
      canvas.clientHeight || canvas.parentElement?.clientHeight || 1,
    );
    renderer.setSize(width, height, false);
    camera.aspect = width / height;
    camera.updateProjectionMatrix();
  };

  const tick = () => {
    if (!active) return;
    raf = window.requestAnimationFrame(tick);
    const t = clock.getElapsedTime();
    sphere.rotation.y = t * 0.45;
    sphere.rotation.x = Math.sin(t * 0.25) * 0.06;
    renderer.render(scene, camera);
  };

  const ro = new ResizeObserver(() => resize());
  ro.observe(canvas);
  resize();
  tick();

  return {
    dispose: () => {
      active = false;
      if (raf) window.cancelAnimationFrame(raf);
      ro.disconnect();
      sphere.geometry.dispose();
      (sphere.material as THREE.Material).dispose();
      ground.geometry.dispose();
      (ground.material as THREE.Material).dispose();
      renderer.dispose();
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        if (active) return;
        active = true;
        resize();
        tick();
      } else {
        active = false;
        if (raf) window.cancelAnimationFrame(raf);
      }
    },
  };
}

const { sections, menu } = setupApp({
  title: "UI src_v1 Fixture",
  debug: true,
});

const sectionEntries: UISharedSectionEntry[] = [
  { template: "docs", sectionID: "ui-home-docs", title: "Home" },
  { template: "table", sectionID: "ui-table-table", title: "Table" },
  { template: "three", sectionID: "ui-three-stage", title: "Three" },
  { template: "terminal", sectionID: "ui-terminal-log", title: "Terminal" },
  { template: "camera", sectionID: "ui-camera-video", title: "Camera" },
];

const sectionByTemplate = new Map<UISharedTemplateID, string>();
const sectionByAlias = new Map<string, string>();
for (const entry of sectionEntries) {
  sectionByTemplate.set(entry.template, entry.sectionID);
  sectionByAlias.set(entry.sectionID, entry.sectionID);
  sectionByAlias.set(entry.template, entry.sectionID);
}

const sectionModes = new Map<string, "fullscreen" | "calculator">();

function applyMode(
  sectionID: string,
  mode: "fullscreen" | "calculator",
): void {
  const section = document.getElementById(sectionID);
  if (!section) return;
  section.classList.remove("fullscreen", "calculator");
  section.classList.add(mode);
  sectionModes.set(sectionID, mode);
}

function toggleModeFor(templateId: UISharedTemplateID, sectionID: string): void {
  const current = sectionModes.get(sectionID) ?? getUISharedTemplate(templateId).defaultMode;
  const next = current === "fullscreen" ? "calculator" : "fullscreen";
  applyMode(sectionID, next);
  console.log(`mode-toggle:${sectionID}:${next}`);
}

function bindInteractions(
  sectionId: UISharedTemplateID,
  sectionID: string,
  container: HTMLElement,
): void {
  if (container.dataset.bound === "1") return;
  container.dataset.bound = "1";

  const modeButton = Array.from(container.querySelectorAll("button")).find(
    (b) => b.textContent?.trim().toLowerCase() === "mode",
  );
  if (modeButton) {
    modeButton.addEventListener("click", () => toggleModeFor(sectionId, sectionID));
  }

  if (sectionId === "table") {
    const refresh = container.querySelector(
      'button[aria-label="Table Refresh"]',
    ) as HTMLButtonElement | null;
    const status = container.querySelector(
      "dd.table-status",
    ) as HTMLElement | null;
    if (refresh && status) {
      refresh.addEventListener("click", () => {
        status.textContent = "refreshed";
        console.log("table-refreshed");
      });
    }
  }

  if (sectionId === "three") {
    const add = container.querySelector(
      'button[aria-label="Three Add"]',
    ) as HTMLButtonElement | null;
    if (add) {
      add.addEventListener("click", () => console.log("three-add:1"));
    }
  }

  if (sectionId === "terminal") {
    const send = container.querySelector(
      'button[aria-label="Terminal Submit"]',
    ) as HTMLButtonElement | null;
    const input = container.querySelector("input") as HTMLInputElement | null;
    const terminal = container.querySelector(".xterm-primary") as HTMLElement | null;
    const status = container.querySelector(
      "dd.terminal-status",
    ) as HTMLElement | null;
    const run = () => {
      const value = (input?.value || "").trim();
      if (!value || !terminal) return;
      terminal.textContent = `${terminal.textContent || ""}\nlog> ${value}`.trim();
      if (status) status.textContent = value;
      console.log(`log-submit:${value}`);
      if (input) input.value = "";
    };
    if (send) send.addEventListener("click", run);
    if (input) {
      input.addEventListener("keydown", (event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          run();
        }
      });
    }
  }
}

registerUISharedSections({
  sections,
  menu,
  entries: sectionEntries,
  decorate: (entry, container) => {
    const template = getUISharedTemplate(entry.template);
    applyMode(entry.sectionID, template.defaultMode);
    bindInteractions(entry.template, entry.sectionID, container);
    const canvas = container.querySelector("canvas");
    if (canvas instanceof HTMLCanvasElement) {
      return mountSphereScene(canvas);
    }
    return ctl();
  },
});

const initialRaw = (window.location.hash || "#ui-home-docs").slice(1).trim().toLowerCase();
const initial = sectionByAlias.get(initialRaw) || sectionByTemplate.get("docs") || "ui-home-docs";
void sections.navigateTo(initial).catch((err) => {
  console.error("[ui-fixture] initial navigation failed", err);
});
