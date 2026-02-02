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

// Interface that all visualization controls must implement
export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

// Configuration for a lazy-loaded section
export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig; // Optional header configuration
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
  }

  /**
   * Register a section for lazy loading
   */
  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
  }

  /**
   * Start observing all registered sections for visibility
   */
  observe(threshold = 0.1): void {
    this.observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const sectionId = entry.target.id;

          if (entry.isIntersecting) {
            // High Priority: Load and show the current section
            this.load(sectionId).then(() => {
              const control = this.visualizations.get(sectionId);
              if (control) {
                control.setVisible(true);
                if (this.debug) {
                  // The component itself will log the "AWAKE" message via VisibilityMixin
                }
              }
            });

            // Update header and subtitle based on current section configuration
            const config = this.configs.get(sectionId);
            const sectionEl = entry.target as HTMLElement;
            this.updateHeader(config?.header, sectionEl);

            // Predictive Priority: Preload next section (but keep it paused)
            const nextId = this.getNextSectionId(sectionId);
            if (nextId) {
              if (this.debug) {
                console.log(`%c[${sectionId}] 🔮 Predictive loading next: ${nextId}`, "color: #94a3b8");
              }
              this.load(nextId);
            }
          } else {
            const control = this.visualizations.get(sectionId);
            if (control) {
              control.setVisible(false);
              if (this.debug) {
                // The component itself will log the "SLEEP" message via VisibilityMixin
              }
            }
          }
        });
      },
      { threshold },
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
   * Eagerly load a section (use for first visible section on page load)
   */
  eagerLoad(sectionId: string): Promise<void> {
    if (this.debug) {
      console.log(
        "%c[SectionManager] 🚀 Eagerly loading first section...",
        "color: #f59e0b",
      );
    }
    const nextId = this.getNextSectionId(sectionId);
    if (nextId) {
      if (this.debug) {
        console.log(`%c[SectionManager] 🔮 Predictive loading second section: ${nextId}`, "color: #94a3b8");
      }
      this.load(nextId);
    }

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

    const startTime = performance.now();
    if (this.debug) {
      console.log(
        `%c[${sectionId}] 📦 Starting lazy load...`,
        "color: #f59e0b; font-weight: bold",
      );
    }

    const loadPromise = config
      .load()
      .then((control) => {
        this.visualizations.set(sectionId, control);
        // Ensure it starts paused (predictive load) until observer wakes it up
        control.setVisible(false);

        if (this.debug) {
          const elapsed = (performance.now() - startTime).toFixed(0);
          console.log(
            `%c[${sectionId}] ✅ Mounted & Paused (${elapsed}ms)`,
            "color: #22c55e; font-weight: bold",
          );
        }
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
    const isVisible = config?.visible ?? true;
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
