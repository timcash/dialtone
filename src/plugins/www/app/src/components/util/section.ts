/**
 * Section Manager - Lazy loading and visibility control for Three.js components
 *
 * Usage:
 *
 * 1. Create a visualization class that implements VisualizationControl:
 *    ```ts
 *    class MyVisualization {
 *        isVisible = true;
 *        frameCount = 0;
 *
 *        setVisible(visible: boolean) {
 *            if (this.isVisible !== visible) {
 *                console.log(`[my-viz] ${visible ? '▶️ Resuming' : '⏸️ Pausing'} at frame ${this.frameCount}`);
 *            }
 *            this.isVisible = visible;
 *        }
 *
 *        dispose() { ... }
 *
 *        animate = () => {
 *            requestAnimationFrame(this.animate);
 *            if (!this.isVisible) return;  // Skip when off-screen
 *            this.frameCount++;
 *            // ... animation logic
 *        }
 *    }
 *    ```
 *
 * 2. Export a mount function:
 *    ```ts
 *    export function mountMyViz(container: HTMLElement): VisualizationControl {
 *        const viz = new MyVisualization(container);
 *        return {
 *            dispose: () => viz.dispose(),
 *            setVisible: (v) => viz.setVisible(v),
 *        };
 *    }
 *    ```
 *
 * 3. Register the section in main.ts:
 *    ```ts
 *    import { SectionManager } from './components/section';
 *
 *    const sections = new SectionManager();
 *
 *    sections.register('s-my-section', {
 *        containerId: 'my-container',
 *        load: async () => {
 *            const { mountMyViz } = await import('./components/my-viz');
 *            const container = document.getElementById('my-container')!;
 *            return mountMyViz(container);
 *        }
 *    });
 *
 *    sections.observe();
 *    sections.eagerLoad('s-home'); // Load first visible section immediately
 *    ```
 */

// Header configuration for a section
export interface HeaderConfig {
  visible?: boolean; // Hide/show the entire header
  title?: string; // Override h1 text
  subtitle?: string; // Override subtitle (priority over data-subtitle)
  telemetry?: boolean; // Show/hide telemetry line
  version?: boolean; // Show/hide version line
}

export interface MenuConfig {
  visible?: boolean; // Hide/show the global menu toggle
}

// Interface that all visualization controls must implement
export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
  updateUI?: () => void; // Optional: Update component-specific UI (like menus)
}

// Configuration for a lazy-loaded section
export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig; // Optional header configuration
  menu?: MenuConfig; // Optional menu configuration
}

/**
 * Manages lazy loading and visibility for Three.js sections
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
  private menuToggleEl: HTMLButtonElement | null = null;
  private menuPanelEl: HTMLElement | null = null;

  constructor(options?: { debug?: boolean }) {
    this.debug = options?.debug ?? true;

    if (this.debug) {
      console.log(
        "%c🌐 SectionManager initialized",
        "color: #22c55e; font-weight: bold; font-size: 14px",
      );
      console.log(
        "%c   Lazy loading: enabled | Animations pause when off-screen",
        "color: #888",
      );
    }

    // Initialize header element references
    this.headerEl = document.querySelector(".header-title");
    this.titleEl = this.headerEl?.querySelector("h1") || null;
    this.subtitleEl = document.getElementById("header-subtitle");
    this.telemetryEl = this.headerEl?.querySelector(".header-telemetry") || null;
    this.versionEl =
      this.headerEl?.querySelector(".version:not(.header-telemetry)") || null;
    this.menuToggleEl = document.getElementById(
      "global-menu-toggle",
    ) as HTMLButtonElement | null;
    this.menuPanelEl = document.getElementById("global-menu-panel");

    // Default hidden unless a section explicitly opts in.
    this.headerEl?.classList.add("is-hidden");
    this.menuToggleEl?.classList.add("is-hidden");
  }

  /**
   * Register a section for lazy loading
   */
  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  private visibilityRatios = new Map<string, number>();
  private activeSectionId: string | null = null;

  /**
   * Start observing all registered sections for visibility
   */
  observe(): void {
    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const sectionId = entry.target.id;
          this.visibilityRatios.set(sectionId, entry.intersectionRatio);

          if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
            // Load only when section is the dominant one
            this.load(sectionId).then(() => {
              const control = this.visualizations.get(sectionId);
              if (control && this.activeSectionId === sectionId) {
                 control.setVisible(true);
              }
            });
          } else if (entry.intersectionRatio < 0.1) {
            // Pause when mostly gone
            const control = this.visualizations.get(sectionId);
            if (control) {
              control.setVisible(false);
            }
          }
        });

        // Determine the "best" section for UI updates (header/menu)
        let bestId: string | null = null;
        let maxRatio = -1;

        this.visibilityRatios.forEach((ratio, id) => {
          if (ratio > maxRatio) {
            maxRatio = ratio;
            bestId = id;
          }
        });

        // Only update UI if the best section has changed and it's substantially visible
        if (bestId && bestId !== this.activeSectionId && maxRatio >= 0.5) {
          const oldId = this.activeSectionId;
          this.activeSectionId = bestId;
          
          if (this.debug) {
            console.log(`%c[SectionManager] 🎯 ACTIVE SECTION: #${bestId} (${(maxRatio * 100).toFixed(0)}%)`, "color: #f59e0b; font-weight: bold");
          }

          // Pause old
          if (oldId) {
              const oldControl = this.visualizations.get(oldId);
              if (oldControl) oldControl.setVisible(false);
          }

          const config = this.configs.get(bestId);
          const sectionEl = document.getElementById(bestId);
          if (sectionEl) {
            this.updateHeader(config?.header, sectionEl);
            this.updateMenu(config?.menu);
            
            // Refresh component-specific UI (like menus) if the method exists
            const control = this.visualizations.get(bestId);
            if (control) {
                if (sectionEl.classList.contains('is-ready')) {
                    control.setVisible(true);
                }
                if (control.updateUI) control.updateUI();
            }
          }
        }
      },
      { threshold: [0, 0.25, 0.5, 0.75, 1.0] },
    );

    const observedSections: string[] = [];
    this.configs.forEach((_, sectionId) => {
      const section = document.getElementById(sectionId);
      if (section) {
        this.observer!.observe(section);
        observedSections.push(sectionId);
      }
    });

    if (this.debug) {
      console.log(
        `%c[SectionManager] 👀 Observing ${observedSections.length} sections:`,
        "color: #06b6d4",
        observedSections.join(", "),
      );
    }
  }

  /**
   * Eagerly load a section
   */
  eagerLoad(sectionId: string): Promise<void> {
    return this.load(sectionId);
  }

  /**
   * Get the ID of the section immediately following the given section
   */
  private getNextSectionId(sectionId: string): string | undefined {
    const keys = Array.from(this.configs.keys());
    const index = keys.indexOf(sectionId);
    if (index !== -1 && index < keys.length - 1) {
      return keys[index + 1];
    }
    return undefined;
  }

  /**
   * Load a visualization (called automatically on visibility, or manually via eagerLoad)
   */
  async load(sectionId: string): Promise<void> {
    // Already loaded
    if (this.visualizations.has(sectionId)) {
      if (this.debug) {
        console.log(`%c[${sectionId}] Already loaded`, "color: #888");
      }
      return;
    }

    // Already loading
    if (this.loadingPromises.has(sectionId)) {
      if (this.debug) {
        console.log(`%c[${sectionId}] Already loading...`, "color: #888");
      }
      return this.loadingPromises.get(sectionId);
    }

    const config = this.configs.get(sectionId);
    if (!config) {
      console.warn(
        `[SectionManager] No config found for section: ${sectionId}`,
      );
      return;
    }

    const sectionEl = document.getElementById(sectionId);
    const loadingBar = sectionEl?.querySelector(".loading-bar") as HTMLElement;
    const updateProgress = (pct: number) => {
      if (loadingBar) loadingBar.style.width = `${pct}%`;
    };

    const startTime = performance.now();
    if (this.debug) {
      console.log(
        `%c[${sectionId}] 📦 Starting lazy load...`,
        "color: #f59e0b; font-weight: bold",
      );
    }

    updateProgress(10);

    const loadPromise = config
      .load()
      .then((control) => {
        updateProgress(90);
        this.visualizations.set(sectionId, control);
        // Ensure it starts paused (predictive load) until observer wakes it up
        control.setVisible(false);

        if (this.debug) {
          const elapsed = (performance.now() - startTime).toFixed(0);
          console.log(
            `%c[SectionManager] ✅ Mounted #${sectionId} (${elapsed}ms)`,
            "color: #22c55e; font-weight: bold",
          );
        }
        
        // Mark as ready
        updateProgress(100);
        setTimeout(() => {
            sectionEl?.classList.add("is-ready");
            console.log(`[SectionManager] READY: #${sectionId}`);
        }, 100);
      })
      .catch((err) => {
        console.error(
          `%c[${sectionId}] ❌ Failed to load:`,
          "color: #ef4444",
          err,
        );
      })
      .finally(() => {
        this.loadingPromises.delete(sectionId);
      });

    this.loadingPromises.set(sectionId, loadPromise);
    return loadPromise;
  }

  /**
   * Check if a section is loaded
   */
  isLoaded(sectionId: string): boolean {
    return this.visualizations.has(sectionId);
  }

  /**
   * Get a visualization control (if loaded)
   */
  get(sectionId: string): VisualizationControl | undefined {
    return this.visualizations.get(sectionId);
  }

  /**
   * Dispose all visualizations and stop observing
   */
  dispose(): void {
    this.observer?.disconnect();
    this.visualizations.forEach((control, id) => {
      if (this.debug) {
        console.log(`%c[${id}] Disposing...`, "color: #888");
      }
      control.dispose();
    });
    this.visualizations.clear();
    this.loadingPromises.clear();
  }

  /**
   * Update the global site header based on section configuration
   */
  private updateHeader(config?: HeaderConfig, sectionEl?: HTMLElement): void {
    if (!this.headerEl) return;

    // 1. Handle Visibility
    const isVisible = config?.visible ?? false;
    this.headerEl.classList.toggle("is-hidden", !isVisible);

    if (!isVisible) return;

    // 2. Handle Title
    if (this.titleEl) {
      this.titleEl.textContent = config?.title || "dialtone.earth";
    }

    // 3. Handle Subtitle (Priority: Config > data-subtitle > default)
    if (this.subtitleEl) {
      const defaultSubtitle = "unified robotic networks for earth";
      const subtitle =
        config?.subtitle || sectionEl?.dataset.subtitle || defaultSubtitle;
      this.subtitleEl.textContent = subtitle;
    }

    // 4. Handle Telemetry & Version visibility
    if (this.telemetryEl) {
      const showTele = config?.telemetry ?? true;
      this.telemetryEl.style.display = showTele ? "block" : "none";
    }

    if (this.versionEl) {
      const showVer = config?.version ?? true;
      this.versionEl.style.display = showVer ? "block" : "none";
    }
  }

  private updateMenu(config?: MenuConfig): void {
    if (!this.menuToggleEl) return;

    const isVisible = config?.visible ?? true;
    this.menuToggleEl.classList.toggle("is-hidden", !isVisible);

    if (!isVisible && this.menuPanelEl) {
      this.menuPanelEl.hidden = true;
      this.menuToggleEl.setAttribute("aria-expanded", "false");
    }
  }
}

/**
 * Helper mixin for visualization classes to add visibility control
 *
 * Usage in your visualization class:
 * ```ts
 * class MyVisualization {
 *     ...VisibilityMixin.defaults(),
 *
 *     setVisible(visible: boolean) {
 *         VisibilityMixin.setVisible(this, visible, 'my-viz');
 *     }
 *
 *     animate = () => {
 *         requestAnimationFrame(this.animate);
 *         if (!this.isVisible) return;
 *         this.frameCount++;
 *         // ...
 *     }
 * }
 * ```
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
  ): void {
    if (target.isVisible !== visible) {
      if (visible) {
        console.log(
          `%c[${name}] ▶️ AWAKE - animation running`,
          "color: #3b82f6; font-weight: bold",
        );
      } else {
        console.log(
          `%c[${name}] 💤 SLEEP - animation paused`,
          "color: #8b5cf6",
        );
      }
    }
    target.isVisible = visible;
  },
};
