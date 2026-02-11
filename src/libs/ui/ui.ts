export * from './types';
export * from './SectionManager';
export * from './Menu';
export * from './FpsCounter';
export * from './GpuTimer';
export * from './Typing';

import { SectionManager } from './SectionManager';
import { Menu } from './Menu';
import { AppOptions } from './types';

// For Vite/Webpack environments, the user can import @ui/style.css in their main.ts
// We also export some common component builders here.

/**
 * Setup the standard Dialtone UI application pattern.
 */
export function setupApp(options: AppOptions) {
  const sections = new SectionManager({ debug: options.debug });
  const menu = Menu.getInstance();

  if (options.title) {
    const titleEl = document.querySelector(".header-title h1");
    if (titleEl) titleEl.textContent = options.title;
  }

  // Global navigation helpers
  (window as any).sections = sections;
  (window as any).navigateTo = (id: string) => sections.navigateTo(id);

  // Sync hash changes
  window.addEventListener('hashchange', () => {
    const id = window.location.hash.slice(1);
    if (id) sections.navigateTo(id);
  });

  return { sections, menu };
}

/**
 * Standard Component Mount Helpers
 */
export const Components = {
  /**
   * Generic Hero Section with typing subtitle
   */
  mountHero: (container: HTMLElement, options: { title: string, subtitles: string[] }) => {
    import('./Typing').then(({ startTyping }) => {
      container.innerHTML = `
        <div class="marketing-overlay">
          <h2>${options.title}</h2>
          <p data-typing-subtitle></p>
        </div>
      `;
      const el = container.querySelector('[data-typing-subtitle]') as HTMLParagraphElement;
      const stop = startTyping(el, options.subtitles);
      (container as any)._stopTyping = stop;
    });

    return {
      dispose: () => {
        if ((container as any)._stopTyping) (container as any)._stopTyping();
        container.innerHTML = '';
      },
      setVisible: () => {}
    };
  },

  /**
   * Generic Docs Section
   */
  mountDocs: (container: HTMLElement, options: { title: string, content: string }) => {
    container.innerHTML = `
      <div class="marketing-overlay">
        <h2>${options.title}</h2>
        <div class="code-block">${options.content}</div>
      </div>
    `;
    return {
      dispose: () => { container.innerHTML = ''; },
      setVisible: () => {}
    };
  }
};