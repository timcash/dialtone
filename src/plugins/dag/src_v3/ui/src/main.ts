import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.dag', debug: true });

sections.register('hit-test', {
  containerId: 'hit-test',
  load: async () => {
    const { mountHitTest } = await import('./components/hit-test/index');
    const container = document.getElementById('hit-test');
    if (!container) throw new Error('hit-test container not found');
    return mountHitTest(container);
  },
  header: { visible: false, menuVisible: true, title: 'DAG Hit Test' },
});

menu.addButton('Hit Test', 'Navigate Hit Test', () => {
  void sections.navigateTo('hit-test');
});

const syncSectionFromURL = () => {
  const hashID = window.location.hash.slice(1);
  const targetID = 'hit-test';
  const activeID = sections.getActiveSectionId();
  if (activeID === targetID) return;
  void sections.navigateTo(targetID, { updateHash: hashID !== targetID }).catch((err) => {
    console.error('[SectionManager] URL sync failed', err);
  });
};

window.addEventListener('hashchange', syncSectionFromURL);
window.addEventListener('pageshow', syncSectionFromURL);
window.addEventListener('focus', syncSectionFromURL);
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) syncSectionFromURL();
});

syncSectionFromURL();
