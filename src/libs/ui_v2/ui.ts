export * from './types';
export * from './SectionManager';
export * from './Menu';

import { Menu } from './Menu';
import { SectionManager } from './SectionManager';
import { AppOptions } from './types';

export function setupApp(options: AppOptions = {}) {
  const sections = new SectionManager({ debug: options.debug ?? true });
  const menu = new Menu();

  if (options.title) {
    const titleEl = document.querySelector('[aria-label="App Header"] h1');
    if (titleEl) titleEl.textContent = options.title;
  }

  (window as any).sections = sections;
  (window as any).navigateTo = (id: string) => sections.navigateTo(id);

  window.addEventListener('hashchange', () => {
    const id = window.location.hash.slice(1);
    if (id) {
      sections.navigateTo(id, { updateHash: false }).catch((err) => {
        console.error('[SectionManager] hash navigation failed', err);
      });
    }
  });

  return { sections, menu };
}
