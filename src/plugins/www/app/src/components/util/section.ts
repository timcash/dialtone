import { Menu } from "./menu";

export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
  updateUI?: () => void;
}

export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: { visible?: boolean; title?: string; subtitle?: string; telemetry?: boolean; version?: boolean };
  menu?: { visible?: boolean };
}

export class SectionManager {
  public visualizations = new Map<string, VisualizationControl>();
  public configs = new Map<string, SectionConfig>();
  public activeSectionId: string | null = null;
  private headerEl: HTMLElement | null = null;
  private menuToggleEl: HTMLButtonElement | null = null;

  constructor() {
    this.headerEl = document.querySelector(".header-title");
    this.menuToggleEl = document.getElementById("global-menu-toggle") as HTMLButtonElement | null;

    // Listen for menu opening to populate it
    window.addEventListener('menu-opening', () => {
      if (this.activeSectionId) {
        const control = this.visualizations.get(this.activeSectionId);
        if (control?.updateUI) control.updateUI();
      }
    });
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  public setActiveSection(sectionId: string): void {
    if (this.activeSectionId === sectionId) return;
    
    // Close menu on section change
    Menu.getInstance().close();

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
      if (control && sectionEl.classList.contains("is-ready")) {
        control.setVisible(true);
      }
    }
  }

  public setLoadingMessage(sectionId: string, message: string): void {
    const sectionEl = document.getElementById(sectionId);
    const messageEl = sectionEl?.querySelector(".loading-message") as HTMLElement;
    if (messageEl) {
      messageEl.textContent = message;
    }
  }

  async load(sectionId: string): Promise<void> {
    if (this.visualizations.has(sectionId)) return;
    const config = this.configs.get(sectionId);
    if (!config) return;

    // Set initial loading message
    this.setLoadingMessage(sectionId, `loading ${sectionId.replace('s-', '')} ...`);

    try {
      // mount is now much faster/sync-like
      const control = await config.load();
      this.visualizations.set(sectionId, control);
      control.setVisible(false);
      
      const sectionEl = document.getElementById(sectionId);
      setTimeout(() => {
        sectionEl?.classList.add("is-ready");
        if (this.activeSectionId === sectionId) control.setVisible(true);
      }, 100);
    } catch (err) {
      console.error(`[SectionManager] Failed to load ${sectionId}:`, err);
      this.setLoadingMessage(sectionId, `error loading ${sectionId}`);
    }
  }

  private updateHeader(config: any, sectionEl: HTMLElement): void {
    if (!this.headerEl) return;
    const visible = config?.visible ?? false;
    this.headerEl.classList.toggle("is-hidden", !visible);
    if (!visible) return;
    const title = this.headerEl.querySelector("h1");
    if (title) title.textContent = config?.title || "dialtone.earth";
    const sub = document.getElementById("header-subtitle");
    if (sub) sub.textContent = config?.subtitle || sectionEl.dataset.subtitle || "";
  }

  private updateMenu(config: any): void {
    if (this.menuToggleEl) {
      this.menuToggleEl.classList.toggle("is-hidden", !(config?.visible ?? true));
    }
  }

  public get(id: string) { return this.visualizations.get(id); }
  public eagerLoad(id: string) { return this.load(id); }
  public observe() { /* IntersectionObserver removed for simplicity in manual nav */ }
}

export const VisibilityMixin = {
  defaults: () => ({ isVisible: true, frameCount: 0 }),
  setVisible(target: any, visible: boolean, name: string): void {
    if (target.isVisible !== visible) {
      console.log(`[${name}] ${visible ? "AWAKE" : "SLEEP"}`);
    }
    target.isVisible = visible;
  },
};
