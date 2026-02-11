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
    if (id) sections.navigateTo(id, { updateHash: false });
  });

  // Live invariants: continuously validate section state transitions and URL/active consistency.
  const validateInvariants = (reason: string) => {
    const snap = sections.getDebugSnapshot();
    const errors: string[] = [];

    const knownIds = new Set(snap.sections.map((s) => s.id));
    const visible = snap.sections.filter((s) => s.domVisible);
    const resumed = snap.sections.filter((s) => s.resumed);

    if (visible.length > 1) {
      errors.push(`more than one section marked visible: ${visible.map((s) => s.id).join(", ")}`);
    }
    if (resumed.length > 1) {
      errors.push(`more than one section resumed: ${resumed.map((s) => s.id).join(", ")}`);
    }

    if (snap.activeSectionId) {
      const active = snap.sections.find((s) => s.id === snap.activeSectionId);
      if (!active) {
        errors.push(`active section "${snap.activeSectionId}" is not registered`);
      } else if (!active.domVisible) {
        errors.push(`active section "${snap.activeSectionId}" is not marked visible`);
      }
    }

    if (snap.hashSectionId && knownIds.has(snap.hashSectionId) && snap.activeSectionId && snap.hashSectionId !== snap.activeSectionId) {
      errors.push(`hash/active mismatch: hash=#${snap.hashSectionId} active=#${snap.activeSectionId}`);
    }

    for (const s of snap.sections) {
      if (s.resumed && !s.loaded) {
        errors.push(`section "${s.id}" resumed before load`);
      }
      if (s.resumed && !s.domVisible) {
        errors.push(`section "${s.id}" resumed while not visible`);
      }
    }

    if (errors.length > 0) {
      for (const err of errors) {
        console.error(`[SectionManager][INVARIANT][${reason}] ${err}`);
      }
    }
  };

  let navCheckTimer: number | null = null;
  const scheduleNavInvariantCheck = (reason: string) => {
    if (navCheckTimer) window.clearTimeout(navCheckTimer);
    navCheckTimer = window.setTimeout(() => validateInvariants(reason), 350);
  };

  window.addEventListener("dialtone:section-navigation", () => scheduleNavInvariantCheck("navigation"));
  window.addEventListener("hashchange", () => scheduleNavInvariantCheck("hashchange"));
  window.setInterval(() => validateInvariants("interval"), 1500);

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
