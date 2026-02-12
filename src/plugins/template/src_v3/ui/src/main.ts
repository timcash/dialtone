import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.template', debug: true });

sections.register('hero', {
  containerId: 'hero',
  load: async () => {
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById('hero');
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3' },
});

sections.register('docs', {
  containerId: 'docs',
  load: async () => {
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById('docs');
    if (!container) throw new Error('docs container not found');
    return mountDocs(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3 Docs' },
});

sections.register('table', {
  containerId: 'table',
  load: async () => {
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById('table');
    if (!container) throw new Error('table container not found');
    return mountTable(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3 Table' },
});

sections.register('three', {
  containerId: 'three',
  load: async () => {
    const { mountThree } = await import('./components/three/index');
    const container = document.getElementById('three');
    if (!container) throw new Error('three container not found');
    return mountThree(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template v3 Three' },
});

sections.register('xterm', {
  containerId: 'xterm',
  load: async () => {
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById('xterm');
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Template v3 Xterm' },
});

sections.register('video', {
  containerId: 'video',
  load: async () => {
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById('video');
    if (!container) throw new Error('video container not found');
    return mountVideo(container);
  },
  header: { visible: true, menuVisible: true, title: 'Template v3 Video' },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo('hero');
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo('docs');
});
menu.addButton('Table', 'Navigate Table', () => {
  void sections.navigateTo('table');
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo('three');
});
menu.addButton('Xterm', 'Navigate Xterm', () => {
  void sections.navigateTo('xterm');
});
menu.addButton('Video', 'Navigate Video', () => {
  void sections.navigateTo('video');
});

const initialId = window.location.hash.slice(1) || 'hero';
console.log(`[SectionManager] INITIAL LOAD #${initialId}`);
void sections.navigateTo(initialId, { updateHash: false });
