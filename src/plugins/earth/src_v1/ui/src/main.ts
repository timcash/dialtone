import { setupApp } from '@ui/ui';
import './style.css';

const { sections, menu } = setupApp({
  title: 'Dialtone Earth',
  debug: true,
});

const SECTION_ID_HERO = 'earth-hero-stage';

sections.register(SECTION_ID_HERO, {
  containerId: SECTION_ID_HERO,
  canonicalName: SECTION_ID_HERO,
  load: async () => {
    sections.setLoadingMessage(SECTION_ID_HERO, 'loading hero ...');
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById(SECTION_ID_HERO);
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Earth Hero' },
  overlays: {
    primaryKind: 'stage',
    primary: '.hero-stage',
    form: '.mode-form',
    legend: '.hero-legend',
  },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo(SECTION_ID_HERO);
});

const hash = window.location.hash.slice(1).trim();
const initial = hash === '' || hash === 'hero' ? SECTION_ID_HERO : hash;
void sections.navigateTo(initial);
