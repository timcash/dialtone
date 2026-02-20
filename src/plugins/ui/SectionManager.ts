import { HeaderConfig, SectionConfig, SectionOverlayConfig, VisualizationControl } from './types';

export class SectionManager {
  private configs = new Map<string, SectionConfig>();
  private controls = new Map<string, VisualizationControl>();
  private loading = new Map<string, Promise<void>>();
  private resumed = new Map<string, boolean>();
  private overlays = new Map<string, Partial<Record<'primary' | 'mode-form' | 'legend' | 'chatlog' | 'status-bar', HTMLElement>>>();
  private activeSectionId: string | null = null;
  private debug: boolean;

  private headerEl: HTMLElement | null;
  private titleEl: HTMLElement | null;
  private menuEl: HTMLElement | null;
  private loadingEl: HTMLElement;

  constructor(options: { debug?: boolean } = {}) {
    this.debug = options.debug ?? true;
    this.headerEl = document.querySelector('[aria-label="App Header"]');
    this.titleEl = this.headerEl?.querySelector('h1') ?? null;
    this.menuEl = document.querySelector('[aria-label="Global Menu"]');
    this.loadingEl = this.ensureLoadingOverlay();
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
    this.bindSectionOverlays(sectionId, config);
  }

  getActiveSectionId(): string | null {
    return this.activeSectionId;
  }

  setLoadingMessage(sectionId: string, message: string): void {
    if (!this.loadingEl) return;
    const span = this.loadingEl.querySelector('span');
    if (span) span.textContent = message;
  }

  isLoaded(sectionId: string): boolean {
    return this.controls.has(sectionId);
  }

  private waitForLayout(): Promise<void> {
    return new Promise((resolve) => {
      window.requestAnimationFrame(() => resolve());
    });
  }

  async load(sectionId: string): Promise<void> {
    if (this.controls.has(sectionId)) return;
    if (this.loading.has(sectionId)) return this.loading.get(sectionId);

    const cfg = this.configs.get(sectionId);
    if (!cfg) throw new Error(`unknown section ${sectionId}`);

    if (this.debug) console.log(`[SectionManager] LOADING #${sectionId}`);
    const p = cfg
      .load()
      .then((ctl) => {
        this.controls.set(sectionId, ctl);
        this.resumed.set(sectionId, false);
        if (this.debug) console.log(`[SectionManager] LOADED #${sectionId}`);
        if (this.debug) console.log(`[SectionManager] START #${sectionId}`);
        ctl.setVisible(false);
      })
      .finally(() => this.loading.delete(sectionId));

    this.loading.set(sectionId, p);
    return p;
  }

  async navigateTo(sectionId: string, options: { updateHash?: boolean } = {}): Promise<void> {
    const updateHash = options.updateHash ?? true;
    const current = this.activeSectionId;
    if (current === sectionId) {
      await this.load(sectionId);
      this.setSectionVisibility(sectionId);
      this.syncOverlayActivity(sectionId);
      this.applyHeader(this.configs.get(sectionId)?.header);
      const ctl = this.controls.get(sectionId);
      if (ctl && !(this.resumed.get(sectionId) ?? false)) {
        await this.waitForLayout();
        ctl.setVisible(true);
        this.resumed.set(sectionId, true);
        if (this.debug) console.log(`[SectionManager] RESUME #${sectionId}`);
      }
      if (updateHash && window.location.hash !== `#${sectionId}`) {
        window.location.hash = `#${sectionId}`;
      }
      return;
    }

    const needsLoad = !this.isLoaded(sectionId);
    if (needsLoad) this.setLoadingVisible(true);
    if (this.debug) console.log(`[SectionManager] NAVIGATING TO #${sectionId}`);
    await this.load(sectionId);
    if (needsLoad) this.setLoadingVisible(false);

    if (current && current !== sectionId) {
      if (this.debug) console.log(`[SectionManager] NAVIGATE AWAY #${current}`);
      const prev = this.controls.get(current);
      if (prev && (this.resumed.get(current) ?? false)) {
        prev.setVisible(false);
        this.resumed.set(current, false);
        if (this.debug) console.log(`[SectionManager] PAUSE #${current}`);
      }
    }

    this.setSectionVisibility(sectionId);
    this.syncOverlayActivity(sectionId);
    this.applyHeader(this.configs.get(sectionId)?.header);
    this.activeSectionId = sectionId;

    if (this.debug) console.log(`[SectionManager] NAVIGATE TO #${sectionId}`);

    const ctl = this.controls.get(sectionId);
    if (ctl && !(this.resumed.get(sectionId) ?? false)) {
      // Let the browser apply visibility/layout before controls read dimensions.
      await this.waitForLayout();
      ctl.setVisible(true);
      this.resumed.set(sectionId, true);
      if (this.debug) console.log(`[SectionManager] RESUME #${sectionId}`);
    }

    if (updateHash && window.location.hash !== `#${sectionId}`) {
      window.location.hash = `#${sectionId}`;
    }
  }

  private setSectionVisibility(activeId: string): void {
    for (const id of this.configs.keys()) {
      const section = document.getElementById(id);
      if (!section) continue;
      if (id === activeId) {
        section.hidden = false;
        section.setAttribute('data-active', 'true');
      } else {
        section.hidden = true;
        section.setAttribute('data-active', 'false');
      }
    }
  }

  private bindSectionOverlays(sectionId: string, cfg: SectionConfig): void {
    const section = document.getElementById(cfg.containerId);
    if (!section) return;
    const overlays: Partial<Record<'primary' | 'mode-form' | 'legend' | 'chatlog' | 'status-bar', HTMLElement>> = {};
    const selectors: SectionOverlayConfig | null = cfg.overlays ?? null;
    if (!selectors) {
      this.overlays.set(sectionId, overlays);
      return;
    }
    const primaryEl = selectors.primary ? section.querySelector(selectors.primary) : null;
    if (primaryEl instanceof HTMLElement) {
      primaryEl.setAttribute('data-overlay', selectors.primaryKind);
      primaryEl.setAttribute('data-overlay-role', 'primary');
      primaryEl.setAttribute('data-overlay-section', sectionId);
      overlays.primary = primaryEl;
    }
    const modeFormSelector = selectors.modeForm ?? selectors.thumb;
    const modeFormEl = modeFormSelector ? section.querySelector(modeFormSelector) : null;
    if (modeFormEl instanceof HTMLElement) {
      modeFormEl.setAttribute('data-overlay', 'mode-form');
      modeFormEl.setAttribute('data-overlay-role', 'mode-form');
      modeFormEl.setAttribute('data-overlay-section', sectionId);
      overlays['mode-form'] = modeFormEl;
    }
    const legendEl = selectors.legend ? section.querySelector(selectors.legend) : null;
    if (legendEl instanceof HTMLElement) {
      legendEl.setAttribute('data-overlay', 'legend');
      legendEl.setAttribute('data-overlay-role', 'legend');
      legendEl.setAttribute('data-overlay-section', sectionId);
      overlays.legend = legendEl;
    }
    const chatlogEl = selectors.chatlog ? section.querySelector(selectors.chatlog) : null;
    if (chatlogEl instanceof HTMLElement) {
      chatlogEl.setAttribute('data-overlay', 'chatlog');
      chatlogEl.setAttribute('data-overlay-role', 'chatlog');
      chatlogEl.setAttribute('data-overlay-section', sectionId);
      overlays.chatlog = chatlogEl;
    }
    const statusBarEl = selectors.statusBar ? section.querySelector(selectors.statusBar) : null;
    if (statusBarEl instanceof HTMLElement) {
      statusBarEl.setAttribute('data-overlay', 'status-bar');
      statusBarEl.setAttribute('data-overlay-role', 'status-bar');
      statusBarEl.setAttribute('data-overlay-section', sectionId);
      overlays['status-bar'] = statusBarEl;
    }
    this.overlays.set(sectionId, overlays);
  }

  private syncOverlayActivity(activeId: string): void {
    for (const [sectionId, sectionOverlays] of this.overlays.entries()) {
      const isActive = sectionId === activeId;
      sectionOverlays.primary?.setAttribute('data-overlay-active', isActive ? 'true' : 'false');
      sectionOverlays['mode-form']?.setAttribute('data-overlay-active', isActive ? 'true' : 'false');
      sectionOverlays.legend?.setAttribute('data-overlay-active', isActive ? 'true' : 'false');
      sectionOverlays.chatlog?.setAttribute('data-overlay-active', isActive ? 'true' : 'false');
      sectionOverlays['status-bar']?.setAttribute('data-overlay-active', isActive ? 'true' : 'false');
    }
    document.body.setAttribute('data-active-section', activeId);
  }

  private ensureLoadingOverlay(): HTMLElement {
    const app = document.getElementById('app') ?? document.body;
    const existing = app.querySelector("[aria-label='Section Loading']");
    if (existing instanceof HTMLElement) return existing;
    const el = document.createElement('div');
    el.classList.add('section-loading');
    el.setAttribute('aria-label', 'Section Loading');
    el.setAttribute('aria-live', 'polite');
    el.hidden = true;
    const text = document.createElement('span');
    text.textContent = 'Loading section...';
    el.appendChild(text);
    app.appendChild(el);
    return el;
  }

  private setLoadingVisible(visible: boolean): void {
    this.loadingEl.hidden = !visible;
    document.body.classList.toggle('section-loading-open', visible);
  }

  private applyHeader(cfg?: HeaderConfig): void {
    if (this.headerEl) this.headerEl.hidden = cfg?.visible === false;
    if (this.menuEl) this.menuEl.hidden = cfg?.menuVisible === false;
    if (cfg?.title && this.titleEl) this.titleEl.textContent = cfg.title;
  }
}
