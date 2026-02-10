export interface HeaderConfig {
  visible?: boolean;
  title?: string;
  subtitle?: string;
}

export interface MenuConfig {
  visible?: boolean;
}

export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig;
  menu?: MenuConfig;
}

export class SectionManager {
  private visualizations = new Map<string, VisualizationControl>();
  private loadingPromises = new Map<string, Promise<void>>();
  private configs = new Map<string, SectionConfig>();
  private observer: IntersectionObserver | null = null;
  private debug: boolean;

  private headerEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private subtitleEl: HTMLElement | null = null;

  constructor(options?: { debug?: boolean }) {
    this.debug = options?.debug ?? false;

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
              if (control) {
                control.setVisible(true);
              }
            });

            const config = this.configs.get(sectionId);
            const sectionEl = entry.target as HTMLElement;
            this.updateHeader(config?.header, sectionEl);
            this.updateMenu(config?.menu);

            const nextId = this.getNextSectionId(sectionId);
            if (nextId) {
              this.load(nextId);
            }
          } else {
            const control = this.visualizations.get(sectionId);
            if (control) {
              control.setVisible(false);
            }
          }
        });
      },
      { threshold }
    );

    this.configs.forEach((_, sectionId) => {
      const section = document.getElementById(sectionId);
      if (section) {
        this.observer!.observe(section);
      }
    });
  }

  eagerLoad(sectionId: string): Promise<void> {
    const nextId = this.getNextSectionId(sectionId);
    if (nextId) {
      this.load(nextId);
    }
    return this.load(sectionId);
  }

  private getNextSectionId(sectionId: string): string | undefined {
    const keys = Array.from(this.configs.keys());
    const index = keys.indexOf(sectionId);
    if (index !== -1 && index < keys.length - 1) {
      return keys[index + 1];
    }
    return undefined;
  }

  async load(sectionId: string): Promise<void> {
    if (this.visualizations.has(sectionId)) {
      return;
    }

    if (this.loadingPromises.has(sectionId)) {
      return this.loadingPromises.get(sectionId);
    }

    const config = this.configs.get(sectionId);
    if (!config) {
      console.warn(`[SectionManager] No config found for section: ${sectionId}`);
      return;
    }

    const loadPromise = config
      .load()
      .then((control) => {
        this.visualizations.set(sectionId, control);
        control.setVisible(false);
      })
      .catch((err) => {
        console.error(`[${sectionId}] Failed to load:`, err);
      })
      .finally(() => {
        this.loadingPromises.delete(sectionId);
      });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  private updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;

    const isVisible = config?.visible ?? true;
    this.headerEl.classList.toggle("is-hidden", !isVisible);
    document.body.classList.toggle("hide-header", !isVisible);

    if (!isVisible) return;

    if (this.titleEl) {
      this.titleEl.textContent = config?.title || "dialtone.dag";
    }

    if (this.subtitleEl) {
      const defaultSubtitle = "nested graph explorer";
      const subtitle = config?.subtitle || sectionEl?.dataset.subtitle || defaultSubtitle;
      this.subtitleEl.textContent = subtitle;
    }
  }

  private updateMenu(config?: MenuConfig): void {
    const isVisible = config?.visible ?? true;
    document.body.classList.toggle("hide-menu", !isVisible);
  }
}
