import { Menu } from "./menu";

export interface HeaderConfig {
  visible?: boolean;
  title?: string;
  subtitle?: string;
  telemetry?: boolean;
  version?: boolean;
}

export interface MenuConfig {
  visible?: boolean;
}

export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
  updateUI?: () => void;
}

export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig;
  menu?: MenuConfig;
}

export class SectionManager {
  public visualizations = new Map<string, VisualizationControl>();
  public loadingPromises = new Map<string, Promise<void>>();
  public configs = new Map<string, SectionConfig>();
  private observer: IntersectionObserver | null = null;

  private headerEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private subtitleEl: HTMLElement | null = null;
  private telemetryEl: HTMLElement | null = null;
  private versionEl: HTMLElement | null = null;
  private menuToggleEl: HTMLButtonElement | null = null;

  public activeSectionId: string | null = null;

  constructor() {
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    this.telemetryEl = this.headerEl?.querySelector(".header-telemetry") || null;
    this.versionEl = this.headerEl?.querySelector(".version:not(.header-telemetry)") || null;
    this.menuToggleEl = document.getElementById("global-menu-toggle") as HTMLButtonElement | null;

    this.headerEl?.classList.add("is-hidden");
    this.menuToggleEl?.classList.add("is-hidden");

    // Simplify Menu Binding: Build on demand when opened
    Menu.getInstance().onOpen(() => {
      if (this.activeSectionId) {
        const control = this.visualizations.get(this.activeSectionId);
        if (control?.updateUI) {
          control.updateUI();
        }
      }
    });
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  public setActiveSection(sectionId: string): void {
    if (this.activeSectionId === sectionId) return;

    // Clear and close menu on section change
    const menu = Menu.getInstance();
    menu.clear();
    menu.close();

    const oldId = this.activeSectionId;
    this.activeSectionId = sectionId;

    if (oldId) {
      const oldControl = this.visualizations.get(oldId);
      if (oldControl) oldControl.setVisible(false);
    }

    const config = this.configs.get(sectionId);
    const sectionEl = document.getElementById(sectionId);
    if (sectionEl) {
      this.updateHeader(config?.header, sectionEl);
      this.updateMenu(config?.menu);

      const control = this.visualizations.get(sectionId);
      if (control) {
        if (sectionEl.classList.contains("is-ready")) {
          control.setVisible(true);
        }
      }
    }
  }

  observe(): void {
    this.observer = new IntersectionObserver((entries) => {
      entries.forEach((entry) => {
        const sectionId = entry.target.id;
        const control = this.visualizations.get(sectionId);
        if (control) {
          if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
            if (this.activeSectionId === sectionId) control.setVisible(true);
          } else if (entry.intersectionRatio < 0.1) {
            control.setVisible(false);
          }
        }
      });
    }, { threshold: [0, 0.25, 0.5, 0.75, 1.0] });

    this.configs.forEach((_, id) => {
      const el = document.getElementById(id);
      if (el) this.observer!.observe(el);
    });
  }

  public get(sectionId: string): VisualizationControl | undefined {
    return this.visualizations.get(sectionId);
  }

  public eagerLoad(sectionId: string): Promise<void> {
    return this.load(sectionId);
  }

  async load(sectionId: string): Promise<void> {
    if (this.visualizations.has(sectionId)) return;
    if (this.loadingPromises.has(sectionId)) return this.loadingPromises.get(sectionId);

    const config = this.configs.get(sectionId);
    if (!config) return;

    const sectionEl = document.getElementById(sectionId);
    const loadingBar = sectionEl?.querySelector(".loading-bar") as HTMLElement;
    
    const loadPromise = config.load().then((control) => {
      if (loadingBar) loadingBar.style.width = "100%";
      this.visualizations.set(sectionId, control);
      control.setVisible(false);
      setTimeout(() => {
        sectionEl?.classList.add("is-ready");
        if (this.activeSectionId === sectionId) control.setVisible(true);
      }, 100);
    });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  public updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;
    const isVisible = config?.visible ?? false;
    this.headerEl.classList.toggle("is-hidden", !isVisible);
    if (!isVisible) return;
    if (this.titleEl) this.titleEl.textContent = config?.title || "dialtone.earth";
    if (this.subtitleEl) {
      this.subtitleEl.textContent = config?.subtitle || sectionEl?.dataset.subtitle || "unified robotic networks for earth";
    }
    if (this.telemetryEl) this.telemetryEl.style.display = (config?.telemetry ?? true) ? "block" : "none";
    if (this.versionEl) this.versionEl.style.display = (config?.version ?? true) ? "block" : "none";
  }

  public updateMenu(config?: MenuConfig): void {
    if (!this.menuToggleEl) return;
    const isVisible = config?.visible ?? true;
    this.menuToggleEl.classList.toggle("is-hidden", !isVisible);
  }

  dispose(): void {
    this.observer?.disconnect();
    this.visualizations.forEach(c => c.dispose());
  }
}

export const VisibilityMixin = {
  defaults: () => ({ isVisible: true, frameCount: 0 }),
  setVisible(target: { isVisible: boolean; frameCount: number }, visible: boolean, name: string): void {
    if (target.isVisible !== visible) {
      console.log(`%c[${name}] ${visible ? "▶️ AWAKE" : "💤 SLEEP"}`, `color: ${visible ? "#3b82f6" : "#8b5cf6"}; font-weight: bold`);
    }
    target.isVisible = visible;
  },
};
