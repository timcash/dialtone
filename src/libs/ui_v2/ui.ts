export * from './types';
export * from './SectionManager';
export * from './Menu';

import { Menu } from './Menu';
import { SectionManager } from './SectionManager';
import { AppOptions } from './types';

function attachThumbButtonHighlight(): void {
  const trigger = (eventTarget: EventTarget | null) => {
    if (!(eventTarget instanceof Element)) return;
    const button = eventTarget.closest("[data-overlay='thumb'] button");
    if (!(button instanceof HTMLButtonElement)) return;
    button.classList.remove('thumb-button-active');
    // Force reflow so repeated taps retrigger animation.
    void button.offsetWidth;
    button.classList.add('thumb-button-active');
  };

  document.addEventListener('click', (event) => {
    trigger(event.target);
  });
}

export function setupApp(options: AppOptions = {}) {
  const sections = new SectionManager({ debug: options.debug ?? true });
  const menu = new Menu();
  attachThumbButtonHighlight();

  if (options.title) {
    const titleEl = document.querySelector('[aria-label="App Header"] h1');
    if (titleEl) titleEl.textContent = options.title;
  }

  (window as any).sections = sections;
  (window as any).navigateTo = (id: string) => sections.navigateTo(id);

  window.addEventListener('hashchange', () => {
    const id = window.location.hash.slice(1);
    if (id && id !== sections.getActiveSectionId()) {
      sections.navigateTo(id, { updateHash: false }).catch((err) => {
        console.error('[SectionManager] hash navigation failed', err);
      });
    }
  });

  return { sections, menu };
}
