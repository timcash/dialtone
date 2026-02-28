import { setupApp } from '../../../../ui/ui/ui';
import { mountLyra3 } from './components/lyra3/index';

const { sections, menu } = setupApp({
  title: 'Lyra3 Music Gen',
  debug: true,
});

sections.register('lyra3', {
  containerId: 'lyra3',
  load: async () => {
    const container = document.getElementById('lyra3');
    if (!container) throw new Error('lyra3 container not found');
    return mountLyra3(container);
  },
  header: {
    title: 'Lyra3 Generation',
    visible: true,
    menuVisible: true,
  },
  overlays: {
    primaryKind: 'button-list',
    primary: '.lyra3-ui',
  },
});

menu.addButton('Lyra3', 'Navigate Lyra3 Gen', () => sections.navigateTo('lyra3'));

void sections.navigateTo('lyra3').catch((err) => {
  console.error('[Lyra3 UI] initial navigation failed', err);
});
