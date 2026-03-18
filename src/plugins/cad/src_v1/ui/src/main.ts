import { setupApp } from '@ui/ui';
import type { VisualizationControl } from '@ui/types';
import './style.css';
import { mountCadStage } from './components/cad/index';

declare const APP_VERSION: string;

const SECTION_ID = 'cad-three-stage';

const { sections, menu } = setupApp({
  title: 'CAD Gear Studio',
  debug: true,
});

const versionEl = document.getElementById('app-version');
if (versionEl) {
  versionEl.textContent = APP_VERSION;
}

sections.register(SECTION_ID, {
  containerId: SECTION_ID,
  canonicalName: SECTION_ID,
  load: async (): Promise<VisualizationControl> => {
    sections.setLoadingMessage(SECTION_ID, 'loading cad stage ...');
    const container = document.getElementById(SECTION_ID);
    if (!container) throw new Error('cad section container not found');
    return mountCadStage(container);
  },
  header: { visible: false, menuVisible: true, title: 'CAD 3D' },
  overlays: {
    primaryKind: 'three',
    primary: '.three-stage',
    form: '.mode-form',
    legend: '.cad-legend',
  },
});

menu.addButton('CAD Stage', 'Navigate CAD 3D stage', () => {
  void sections.navigateTo(SECTION_ID);
});

void sections.navigateTo(SECTION_ID).catch((err) => {
  console.error('[cad/ui] initial navigation failed', err);
});
