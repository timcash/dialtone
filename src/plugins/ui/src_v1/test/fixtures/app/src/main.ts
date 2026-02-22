import { setupApp } from '../../../../ui/ui';
import '../../../../ui/style.css';
import './style.css';

function ctl(): { dispose: () => void; setVisible: (visible: boolean) => void } {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {}
  };
}

const { sections, menu } = setupApp({ title: 'UI src_v1 Fixture', debug: true });

sections.register('ui-hero-stage', {
  containerId: 'ui-hero-stage',
  load: async () => {
    const status = document.querySelector("[aria-label='Hero Status']") as HTMLElement | null;
    if (status) status.setAttribute('data-ready', 'true');
    return ctl();
  },
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Hero Canvas']",
    legend: "[aria-label='Hero Status']"
  }
});

sections.register('ui-docs-docs', {
  containerId: 'ui-docs-docs',
  load: async () => ctl(),
  overlays: {
    primaryKind: 'docs',
    primary: "[aria-label='Docs Content']"
  }
});

sections.register('ui-meta-table', {
  containerId: 'ui-meta-table',
  load: async () => {
    const status = document.querySelector("[aria-label='Table Status']") as HTMLElement | null;
    const refresh = document.querySelector("button[aria-label='Table Thumb 1']") as HTMLButtonElement | null;
    if (refresh && !refresh.dataset.bound) {
      refresh.dataset.bound = '1';
      refresh.addEventListener('click', () => {
        if (status) {
          status.setAttribute('data-state', 'refreshed');
          status.textContent = 'refreshed';
        }
        console.log('table-refreshed');
      });
    }
    return ctl();
  },
  overlays: {
    primaryKind: 'table',
    primary: "table[aria-label='UI Table']",
    modeForm: "form[data-mode-form='table']",
    statusBar: "[aria-label='Table Status']"
  }
});

sections.register('ui-three-stage', {
  containerId: 'ui-three-stage',
  load: async () => {
    const countEl = document.querySelector("[aria-label='Three Count']") as HTMLElement | null;
    const add = document.querySelector("button[aria-label='Three Add']") as HTMLButtonElement | null;
    if (add && !add.dataset.bound) {
      add.dataset.bound = '1';
      add.addEventListener('click', () => {
        const curr = Number(countEl?.getAttribute('data-count') || '0');
        const next = curr + 1;
        if (countEl) {
          countEl.setAttribute('data-count', String(next));
          countEl.textContent = String(next);
        }
        console.log(`three-add:${next}`);
      });
    }
    return ctl();
  },
  overlays: {
    primaryKind: 'stage',
    primary: "canvas[aria-label='Three Canvas']",
    modeForm: "form[data-mode-form='three']",
    statusBar: "[aria-label='Three Count']"
  }
});

sections.register('ui-log-xterm', {
  containerId: 'ui-log-xterm',
  load: async () => {
    const terminal = document.querySelector("[aria-label='Log Terminal']") as HTMLElement | null;
    const input = document.querySelector("input[aria-label='Log Input']") as HTMLInputElement | null;
    if (input && !input.dataset.bound) {
      input.dataset.bound = '1';
      input.addEventListener('keydown', (event) => {
        if (event.key !== 'Enter') return;
        const val = (input.value || '').trim();
        if (terminal) {
          terminal.setAttribute('data-last', val);
          terminal.textContent = val;
        }
        console.log(`log-submit:${val}`);
      });
    }
    return ctl();
  },
  overlays: {
    primaryKind: 'xterm',
    primary: "[aria-label='Log Terminal']"
  }
});

sections.register('ui-demo-video', {
  containerId: 'ui-demo-video',
  load: async () => ctl(),
  overlays: {
    primaryKind: 'video',
    primary: "video[aria-label='Test Video']"
  }
});

menu.addButton('Hero', 'Navigate Hero', () => sections.navigateTo('ui-hero-stage'));
menu.addButton('Docs', 'Navigate Docs', () => sections.navigateTo('ui-docs-docs'));
menu.addButton('Table', 'Navigate Table', () => sections.navigateTo('ui-meta-table'));
menu.addButton('Stage', 'Navigate Stage', () => sections.navigateTo('ui-three-stage'));
menu.addButton('Log', 'Navigate Log', () => sections.navigateTo('ui-log-xterm'));
menu.addButton('Video', 'Navigate Video', () => sections.navigateTo('ui-demo-video'));

const initial = (window.location.hash || '#ui-hero-stage').slice(1);
sections.navigateTo(initial || 'ui-hero-stage').catch((err) => {
  console.error('[ui-fixture] initial navigation failed', err);
});
