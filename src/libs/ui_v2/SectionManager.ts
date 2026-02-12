import { HeaderConfig, SectionConfig, VisualizationControl } from './types';

export class SectionManager {
  private configs = new Map<string, SectionConfig>();
  private controls = new Map<string, VisualizationControl>();
  private loading = new Map<string, Promise<void>>();
  private resumed = new Map<string, boolean>();
  private activeSectionId: string | null = null;
  private debug: boolean;

  private headerEl: HTMLElement | null;
  private titleEl: HTMLElement | null;
  private menuEl: HTMLElement | null;

  constructor(options: { debug?: boolean } = {}) {
    this.debug = options.debug ?? true;
    this.headerEl = document.querySelector('[aria-label="App Header"]');
    this.titleEl = this.headerEl?.querySelector('h1') ?? null;
    this.menuEl = document.querySelector('[aria-label="Global Menu"]');
  }

  register(sectionId: string, config: SectionConfig): void {
    this.configs.set(sectionId, config);
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
    if (this.debug) console.log(`[SectionManager] NAVIGATING TO #${sectionId}`);

    const current = this.activeSectionId;
    await this.load(sectionId);

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
    this.applyHeader(this.configs.get(sectionId)?.header);
    this.activeSectionId = sectionId;

    if (this.debug) console.log(`[SectionManager] NAVIGATE TO #${sectionId}`);

    const ctl = this.controls.get(sectionId);
    if (ctl && !(this.resumed.get(sectionId) ?? false)) {
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

  private applyHeader(cfg?: HeaderConfig): void {
    if (this.headerEl) this.headerEl.hidden = cfg?.visible === false;
    if (this.menuEl) this.menuEl.hidden = cfg?.menuVisible === false;
    if (cfg?.title && this.titleEl) this.titleEl.textContent = cfg.title;
  }
}
