export interface HeaderConfig {
  visible?: boolean;
  menuVisible?: boolean; // New option to control global menu
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
  private menuEl: HTMLElement | null = null;

  constructor(_options?: { debug?: boolean }) {
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    this.menuEl = document.getElementById("global-menu");
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  async navigateTo(sectionId: string, smooth = true) {
    console.log(`[SectionManager] 🚀 Navigating to: #${sectionId}`);
    
    await this.load(sectionId);
    
    const config = this.configs.get(sectionId);
    const el = document.getElementById(sectionId);
    
    if (el) {
        // Update header/menu state immediately
        this.updateHeader(config?.header, el);
        
        el.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: 'start' });
        
        // Wait for scroll to finish then emit event
        const delay = smooth ? 800 : 50;
        setTimeout(() => {
            window.dispatchEvent(new CustomEvent('section-nav-complete', { detail: { sectionId } }));
        }, delay);
    }
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
    if (this.headerEl) {
        const isVisible = config?.visible ?? true;
        this.headerEl.classList.toggle("is-hidden", !isVisible);
        if (isVisible) {
            if (this.titleEl) this.titleEl.textContent = config?.title || "dialtone.nix";
            if (this.subtitleEl) {
                this.subtitleEl.textContent = config?.subtitle || sectionEl?.dataset.subtitle || "distributed data explorer";
            }
        }
    }

    const isMenuVisible = config?.menuVisible ?? true;
    import("./menu").then(({ Menu }) => {
        Menu.getInstance().setVisible(isMenuVisible);
    });
  }
}

export const VisibilityMixin = {
  defaults: () => ({ isVisible: true, frameCount: 0 }),
  setVisible(target: { isVisible: boolean; frameCount: number }, visible: boolean, _name: string): void {
    target.isVisible = visible;
  },
};
