import { setupApp } from "../../../../ui/ui";
import {
  getUISharedTemplate,
  renderUISharedTemplate,
  type UISharedTemplateID,
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

const sectionIDs: UISharedTemplateID[] = [
  "hero",
  "three-fullscreen",
  "three-calculator",
  "table",
  "camera",
  "docs",
  "terminal",
  "settings",
];

const sectionModes = new Map<UISharedTemplateID, "fullscreen" | "calculator">();

function applyMode(
  sectionId: UISharedTemplateID,
  mode: "fullscreen" | "calculator",
): void {
  const section = document.getElementById(sectionId);
  if (!section) return;
  section.classList.remove("fullscreen", "calculator");
  section.classList.add(mode);
  sectionModes.set(sectionId, mode);
}

function toggleModeFor(sectionId: UISharedTemplateID): void {
  const current =
    sectionModes.get(sectionId) ?? getUISharedTemplate(sectionId).defaultMode;
  const next = current === "fullscreen" ? "calculator" : "fullscreen";
  applyMode(sectionId, next);
  console.log(`mode-toggle:${sectionId}:${next}`);
}

function activeSectionId(): UISharedTemplateID {
  const id = sections.getActiveSectionId() as UISharedTemplateID | null;
  if (id && sectionIDs.includes(id)) return id;
  return "hero";
}

function bindInteractions(
  sectionId: UISharedTemplateID,
  container: HTMLElement,
): void {
  if (container.dataset.bound === "1") return;
  container.dataset.bound = "1";

  const modeButton = Array.from(container.querySelectorAll("button")).find(
    (b) => b.textContent?.trim().toLowerCase() === "mode",
  );
  if (modeButton) {
    modeButton.addEventListener("click", () => toggleModeFor(sectionId));
  }

  if (sectionId === "table") {
    const refresh = container.querySelector(
      "button.table-refresh",
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

  if (sectionId === "three-calculator") {
    const add = container.querySelector(
      "button.three-add",
    ) as HTMLButtonElement | null;
    if (add) {
      add.addEventListener("click", () => console.log("three-add:1"));
    }
  }

  if (sectionId === "terminal") {
    const send = container.querySelector(
      "button.terminal-send",
    ) as HTMLButtonElement | null;
    const input = container.querySelector("input") as HTMLInputElement | null;
    const pre = container.querySelector("pre") as HTMLElement | null;
    const status = container.querySelector(
      "dd.terminal-status",
    ) as HTMLElement | null;
    const run = () => {
      const value = (input?.value || "").trim();
      if (!value || !pre) return;
      pre.textContent += `log> ${value}\n`;
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

for (const sectionId of sectionIDs) {
  const template = getUISharedTemplate(sectionId);
  sections.register(sectionId, {
    containerId: sectionId,
    load: async () => {
      const container = document.getElementById(sectionId);
      if (!container) throw new Error(`${sectionId} container not found`);
      renderUISharedTemplate(container, sectionId);
      applyMode(sectionId, template.defaultMode);
      bindInteractions(sectionId, container);
      const canvas = container.querySelector("canvas");
      if (canvas instanceof HTMLCanvasElement) {
        return mountSphereScene(canvas);
      }
      return ctl();
    },
    overlays: template.overlays,
  });

  menu.addButton(template.title, `Navigate ${template.title}`, () =>
    sections.navigateTo(sectionId),
  );
}

menu.addButton("Layout", "Toggle Layout Mode", () => {
  toggleModeFor(activeSectionId());
});

window.addEventListener("keydown", (event) => {
  if (event.defaultPrevented) return;
  if (event.metaKey || event.ctrlKey || event.altKey) return;
  if (event.key.toLowerCase() !== "m") return;
  const target = event.target as HTMLElement | null;
  if (target && ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName))
    return;
  event.preventDefault();
  toggleModeFor(activeSectionId());
});

const initialRaw = (window.location.hash || "#hero").slice(1);
const initial = sectionIDs.includes(initialRaw as UISharedTemplateID)
  ? (initialRaw as UISharedTemplateID)
  : "hero";
void sections.navigateTo(initial).catch((err) => {
  console.error("[ui-fixture] initial navigation failed", err);
});
