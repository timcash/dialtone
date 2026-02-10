export * from './types';
export * from './menu';
import { VisualizationControl, SectionConfig, HeaderConfig } from './types';
import { Menu } from './menu';

/**
 * SectionManager handles lazy loading, visibility, and header updates for Three.js sections.
 */
export class SectionManager {
  private visualizations = new Map<string, VisualizationControl>();
  private loadingPromises = new Map<string, Promise<void>>();
  private configs = new Map<string, SectionConfig>();
  private observer: IntersectionObserver | null = null;
  private debug: boolean;

  // Header element references
  private headerEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private subtitleEl: HTMLElement | null = null;

  constructor(options: { title?: string; debug?: boolean } = {}) {
    this.debug = options.debug ?? true;
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    
    if (this.titleEl && options.title) {
        this.titleEl.textContent = options.title;
    }
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
            this.updateHeader(config?.header, entry.target as HTMLElement);
          } else {
            const control = this.visualizations.get(sectionId);
            if (control) {
              control.setVisible(false);
            }
          }
        });
      },
      { threshold },
    );

    this.configs.forEach((_, sectionId) => {
      const section = document.getElementById(sectionId);
      if (section) {
        this.observer!.observe(section);
      }
    });
  }

  async load(sectionId: string): Promise<void> {
    if (this.visualizations.has(sectionId)) return;
    if (this.loadingPromises.has(sectionId)) return this.loadingPromises.get(sectionId);

    const config = this.configs.get(sectionId);
    if (!config) return;

    const loadPromise = config
      .load()
      .then((control) => {
        this.visualizations.set(sectionId, control);
        control.setVisible(false);
      })
      .finally(() => {
        this.loadingPromises.delete(sectionId);
      });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  async navigateTo(id: string) {
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  }

  private updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;

    const isVisible = config?.visible ?? true;
    this.headerEl.classList.toggle("is-hidden", !isVisible);

    if (!isVisible) return;

    if (this.titleEl) {
      this.titleEl.textContent = config?.title || this.titleEl.textContent;
    }

    if (this.subtitleEl) {
      const defaultSubtitle = "plugin architecture template";
      const subtitle = config?.subtitle || sectionEl?.dataset.subtitle || defaultSubtitle;
      this.subtitleEl.textContent = subtitle;
    }
  }
}

export const VisibilityMixin = {
  defaults: () => ({
    isVisible: true,
    frameCount: 0,
  }),

  setVisible(
    target: { isVisible: boolean; frameCount: number },
    visible: boolean,
    name: string,
  ): void {
    if (target.isVisible !== visible) {
      if (this.debug) {
          console.log(`[${name}] ${visible ? 'AWAKE' : 'SLEEP'}`);
      }
    }
    target.isVisible = visible;
  },
};

/**
 * setupApp is a helper to initialize the standard UI patterns.
 */
export function setupApp(options: { title: string }) {
    const sections = new SectionManager({ title: options.title });
    const menu = Menu.getInstance();

    (window as any).sections = sections;
    (window as any).navigateTo = (id: string) => sections.navigateTo(id);

    window.addEventListener('hashchange', () => {
        const id = window.location.hash.slice(1);
        if (id) sections.navigateTo(id);
    });

    return { sections, menu };
}