import { SectionComponent, SectionConfig } from './types';

export class SectionManager {
  private components = new Map<string, SectionComponent>();
  private configs = new Map<string, SectionConfig>();

  register(id: string, config: SectionConfig) {
    this.configs.set(id, config);
  }

  async navigateTo(id: string) {
    const config = this.configs.get(id);
    const el = document.getElementById(id);
    if (!el || !config) return;

    // Toggle header/menu visibility via body classes
    document.body.classList.toggle('hide-header', config.header?.visible === false);
    document.body.classList.toggle('hide-menu', config.header?.menuVisible === false);

    if (!this.components.has(id)) {
      const comp = new config.component(el);
      await comp.mount();
      this.components.set(id, comp);
    }

    document.querySelectorAll('section').forEach(s => s.classList.remove('is-active'));
    el.classList.add('is-active');
    
    // Smooth scroll to section
    el.scrollIntoView({ behavior: 'smooth' });
  }
}
