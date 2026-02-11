import { SectionConfig, HeaderConfig, VisualizationControl } from './types';

/**
 * Manages lazy loading and visibility for UI sections.
 * Optimized for Three.js and other high-performance visualizations.
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
  private telemetryEl: HTMLElement | null = null;
  private versionEl: HTMLElement | null = null;
  private menuEl: HTMLElement | null = null;
  private activeSectionId: string | null = null;
  private resumed = new Map<string, boolean>();

  constructor(options: { debug?: boolean } = {}) {
    this.debug = options.debug ?? true;

    // Cache DOM references
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    this.telemetryEl = this.headerEl?.querySelector(".header-telemetry") || null;
    this.versionEl = this.headerEl?.querySelector(".version:not(.header-telemetry)") || null;
    this.menuEl = document.querySelector(".top-right-controls");
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  observe(threshold = 0.1): void {
    if (this.observer) this.observer.disconnect();

    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const sectionId = entry.target.id;

          if (entry.isIntersecting) {
            this.setActiveSection(sectionId);

            // Toggle visibility class for CSS animations
            entry.target.classList.add('is-visible');

            this.load(sectionId).then(() => {
              const control = this.visualizations.get(sectionId);
              if (control && !this.resumed.get(sectionId)) {
                if (this.debug) console.log(`[SectionManager] 🚀 RESUME #${sectionId}`);
                control.setVisible(true);
                this.resumed.set(sectionId, true);
              }
            });

            const config = this.configs.get(sectionId);
            this.updateHeader(config?.header, entry.target as HTMLElement);

            // Preload next section
            const nextId = this.getNextSectionId(sectionId);
            if (nextId) this.load(nextId);
          } else {
            entry.target.classList.remove('is-visible');
            if (this.activeSectionId === sectionId) this.setActiveSection(null);
            const control = this.visualizations.get(sectionId);
            if (control && this.resumed.get(sectionId)) {
              if (this.debug) console.log(`[SectionManager] 💤 PAUSE #${sectionId}`);
              control.setVisible(false);
              this.resumed.set(sectionId, false);
            }
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

    if (this.debug) console.log(`[SectionManager] 📦 LOADING #${sectionId}...`);
    const startTime = performance.now();

    const loadPromise = config
      .load()
      .then((control) => {
        const elapsed = (performance.now() - startTime).toFixed(0);
        if (this.debug) console.log(`[SectionManager] ✅ LOADED #${sectionId} (${elapsed}ms)`);

        this.visualizations.set(sectionId, control);
        // Start phase: initial setup but paused until visible
        if (this.debug) console.log(`[SectionManager] ✨ START #${sectionId}`);
        control.setVisible(false);
        this.resumed.set(sectionId, false);
      })
      .catch((err) => {
        console.error(`[SectionManager] ❌ FAILED to load #${sectionId}:`, err);
      })
      .finally(() => {
        this.loadingPromises.delete(sectionId);
      });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  eagerLoad(sectionId: string): Promise<void> {
    return this.load(sectionId);
  }

  async navigateTo(id: string, options: { updateHash?: boolean } = {}) {
    const updateHash = options.updateHash ?? true;
    if (this.debug) console.log(`[SectionManager] 🧭 NAVIGATING TO #${id}`);
    if (updateHash && window.location.hash !== `#${id}`) {
      window.location.hash = `#${id}`;
    }
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth' });
    }
  }

  private setActiveSection(nextId: string | null): void {
    if (this.activeSectionId === nextId) return;

    if (this.activeSectionId) {
      if (this.debug) console.log(`[SectionManager] 🧭 NAVIGATE AWAY #${this.activeSectionId}`);
    }
    if (nextId) {
      if (this.debug) console.log(`[SectionManager] 🧭 NAVIGATE TO #${nextId}`);
    }
    this.activeSectionId = nextId;

    window.dispatchEvent(new CustomEvent("dialtone:section-navigation", {
      detail: {
        activeSectionId: this.activeSectionId,
      },
    }));
  }

  getDebugSnapshot(): {
    activeSectionId: string | null;
    hashSectionId: string;
    sections: Array<{ id: string; loaded: boolean; resumed: boolean; domVisible: boolean; loading: boolean }>;
  } {
    const hashSectionId = window.location.hash.slice(1);
    const sections = Array.from(this.configs.keys()).map((id) => {
      const el = document.getElementById(id);
      return {
        id,
        loaded: this.visualizations.has(id),
        resumed: this.resumed.get(id) ?? false,
        domVisible: !!el?.classList.contains("is-visible"),
        loading: this.loadingPromises.has(id),
      };
    });
    return {
      activeSectionId: this.activeSectionId,
      hashSectionId,
      sections,
    };
  }

  private getNextSectionId(sectionId: string): string | undefined {
    const keys = Array.from(this.configs.keys());
    const index = keys.indexOf(sectionId);
    if (index !== -1 && index < keys.length - 1) {
      return keys[index + 1];
    }
    return undefined;
  }

  private updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;

    const isVisible = config?.visible ?? true;
    this.headerEl.classList.toggle("is-hidden", !isVisible);
    this.headerEl.style.display = isVisible ? "flex" : "none";

    if (!isVisible) return;

    if (this.titleEl && config?.title) {
      this.titleEl.textContent = config.title;
    }

    if (this.subtitleEl) {
      const subtitle = config?.subtitle || sectionEl?.dataset.subtitle || "";
      this.subtitleEl.textContent = subtitle;
    }

    if (this.telemetryEl) {
      this.telemetryEl.style.display = config?.telemetry === false ? "none" : "block";
    }

    if (this.versionEl) {
      this.versionEl.style.display = config?.version === false ? "none" : "block";
    }

    if (this.menuEl) {
      const isMenuVisible = config?.menuVisible ?? true;
      this.menuEl.style.display = isMenuVisible ? "flex" : "none";
    }
  }

  dispose(): void {
    this.observer?.disconnect();
    this.visualizations.forEach(v => v.dispose());
    this.visualizations.clear();
    this.resumed.clear();
  }
}

/**
 * Standard visibility mixin for visualization classes
 */
export const VisibilityMixin = {
  defaults: () => ({
    isVisible: true,
    frameCount: 0,
  }),

  setVisible(
    target: { isVisible: boolean; frameCount: number },
    visible: boolean,
    name: string,
    debug: boolean = true
  ): void {
    if (target.isVisible !== visible) {
      if (debug) {
        console.log(`[${name}] ${visible ? 'AWAKE' : 'SLEEP'}`);
      }
    }
    target.isVisible = visible;
  },
};
