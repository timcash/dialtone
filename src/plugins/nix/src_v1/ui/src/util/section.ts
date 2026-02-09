export interface HeaderConfig {
  visible?: boolean;
  title?: string;
  subtitle?: string;
  telemetry?: boolean;
  version?: boolean;
}

export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig;
}

export class SectionManager {
  private visualizations = new Map<string, VisualizationControl>();
  private loadingPromises = new Map<string, Promise<void>>();
  private configs = new Map<string, SectionConfig>();
  private observer: IntersectionObserver | null = null;

  private headerEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private subtitleEl: HTMLElement | null = null;

  constructor(_options?: { debug?: boolean }) {
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  observe(threshold = 0.1): void {
    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const sectionId = entry.target.id;
          if (entry.isIntersecting) {
            this.load(sectionId).then(() => {
              const control = this.visualizations.get(sectionId);
              if (control) control.setVisible(true);
            });
            const config = this.configs.get(sectionId);
            this.updateHeader(config?.header, entry.target as HTMLElement);
          } else {
            const control = this.visualizations.get(sectionId);
            if (control) control.setVisible(false);
          }
        });
      },
      { threshold },
    );

    this.configs.forEach((_, sectionId) => {
      const section = document.getElementById(sectionId);
      if (section) this.observer!.observe(section);
    });
  }

  async load(sectionId: string): Promise<void> {
    if (this.visualizations.has(sectionId)) return;
    if (this.loadingPromises.has(sectionId)) return this.loadingPromises.get(sectionId);

    const config = this.configs.get(sectionId);
    if (!config) return;

    const loadPromise = config.load().then((control) => {
      this.visualizations.set(sectionId, control);
      control.setVisible(false);
    }).finally(() => {
      this.loadingPromises.delete(sectionId);
    });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  dispose(): void {
    this.observer?.disconnect();
    this.visualizations.forEach((control) => control.dispose());
    this.visualizations.clear();
  }

  private updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;
    const isVisible = config?.visible ?? true;
    this.headerEl.classList.toggle("is-hidden", !isVisible);
    if (!isVisible) return;
    if (this.titleEl) this.titleEl.textContent = config?.title || "dialtone.swarm";
    if (this.subtitleEl) {
      this.subtitleEl.textContent = config?.subtitle || sectionEl?.dataset.subtitle || "distributed data explorer";
    }
  }
}

export const VisibilityMixin = {
  defaults: () => ({ isVisible: true, frameCount: 0 }),
  setVisible(target: { isVisible: boolean; frameCount: number }, visible: boolean, _name: string): void {
    target.isVisible = visible;
  },
};
