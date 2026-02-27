import { setupApp } from '@ui/ui';
import './style.css';

const { sections, menu } = setupApp({
  title: 'Dialtone Earth',
  debug: true,
});

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    sections.setLoadingMessage('hero', 'loading hero ...');
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Earth Hero' },
  overlays: {
    primaryKind: 'stage',
    primary: '.hero-stage',
    thumb: '.mode-form',
    legend: '.hero-legend',
  },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('hero');
});

void sections.navigateTo(window.location.hash.slice(1) || 'hero');
