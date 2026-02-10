import { SectionComponent, SectionConfig, HeaderConfig } from './types';
export * from './types';
export * from './menu';
import { Menu } from './menu';

/**
 * SectionManager handles lazy loading, visibility, and header updates for SPA sections.
 */
export class SectionManager {
  private components = new Map<string, SectionComponent>();
  private configs = new Map<string, SectionConfig>();
  private observer: IntersectionObserver | null = null;

  // Header element references
  private headerEl: HTMLElement | null = null;
  private titleEl: HTMLElement | null = null;
  private subtitleEl: HTMLElement | null = null;

  constructor(options: { title?: string } = {}) {
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    if (this.titleEl && options.title) {
        this.titleEl.textContent = options.title;
    }
  }

  register(id: string, config: SectionConfig) {
    this.configs.set(id, config);
  }

  /**
   * observe setup intersection observer to automatically mount/unmount and show/hide sections.
   */
  observe(threshold = 0.1): void {
    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const id = entry.target.id;
          const config = this.configs.get(id);
          
          if (entry.isIntersecting) {
            this.mountAndShow(id);
            this.updateHeader(config?.header, entry.target as HTMLElement);
          } else {
            const comp = this.components.get(id);
            if (comp) {
                comp.setVisible(false);
                // Optional: Clear menu if this section had one
                // Menu.getInstance().clear(); 
            }
          }
        });
      },
      { threshold }
    );

    this.configs.forEach((_, id) => {
      const el = document.getElementById(id);
      if (el) this.observer!.observe(el);
    });
  }

  async mountAndShow(id: string) {
    const config = this.configs.get(id);
    const el = document.getElementById(id);
    if (!el || !config) return;

    if (!this.components.has(id)) {
      const comp = new config.component(el);
      await comp.mount();
      this.components.set(id, comp);
    }

    const comp = this.components.get(id);
    if (comp) comp.setVisible(true);

    document.querySelectorAll('.snap-slide').forEach(s => s.classList.remove('is-active'));
    el.classList.add('is-active');
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

    // Use default title if none provided in config
    if (this.titleEl) {
      this.titleEl.textContent = config?.title || this.titleEl.textContent;
    }

    if (this.subtitleEl) {
      const subtitle = config?.subtitle || sectionEl?.dataset.subtitle || "";
      this.subtitleEl.textContent = subtitle;
    }
  }
}

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
