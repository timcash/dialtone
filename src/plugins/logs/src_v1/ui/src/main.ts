import { setupApp } from '../../../../../libs/ui_v2/ui';
import './style.css';

const { sections, menu } = setupApp({ title: 'dialtone.logs', debug: true });

sections.register('logs-log-xterm', {
  containerId: 'logs-log-xterm',
  load: async () => {
    const { mountLog } = await import('./components/log/index');
    const container = document.getElementById('logs-log-xterm');
    if (!container) throw new Error('logs-log-xterm container not found');
    return mountLog(container);
  },
  header: { visible: false, menuVisible: true, title: 'Logs' },
  overlays: {
    primaryKind: 'xterm',
    primary: "[aria-label='Log Terminal']",
    modeForm: "form[data-mode-form='log']",
    legend: '.logs-log-legend',
  },
});

const sectionSet = new Set(['logs-log-xterm']);
const sectionOrder = ['logs-log-xterm'] as const;
type LogsSectionID = (typeof sectionOrder)[number];
const defaultSection: LogsSectionID = 'logs-log-xterm';
const sectionStorageKey = 'logs.src_v1.active_section';

const readStoredSection = (): LogsSectionID | null => {
  try {
    const value = window.sessionStorage.getItem(sectionStorageKey);
    if (!value) return null;
    return sectionSet.has(value) ? (value as LogsSectionID) : null;
  } catch {
    return null;
  }
};

const readHashSection = (): LogsSectionID | null => {
  const hashID = window.location.hash.slice(1);
  return sectionSet.has(hashID) ? (hashID as LogsSectionID) : null;
};

const writeStoredSection = (sectionId: LogsSectionID) => {
  try {
    window.sessionStorage.setItem(sectionStorageKey, sectionId);
  } catch {
    // ignore
  }
};

const isSectionActuallyVisible = (sectionId: LogsSectionID): boolean => {
  const section = document.getElementById(sectionId);
  if (!section) return false;
  return !section.hidden && section.getAttribute('data-active') === 'true';
};

const navigateToSection = (sectionId: LogsSectionID, updateHash = true) => {
  const active = sections.getActiveSectionId() as LogsSectionID | null;
  const activeLooksWrong = active === sectionId && !isSectionActuallyVisible(sectionId);
  if (activeLooksWrong) {
    return sections.navigateTo(sectionId, { updateHash }).then(() => {
      writeStoredSection(sectionId);
    });
  }
  return sections.navigateTo(sectionId, { updateHash }).then(() => {
    writeStoredSection(sectionId);
  });
};

menu.addButton('Log', 'Navigate Log', () => {
  void navigateToSection('logs-log-xterm');
});

const syncSectionFromURL = () => {
  const hashID = readHashSection();
  const storedSection = readStoredSection();
  const targetID = hashID ?? storedSection ?? defaultSection;
  const activeID = sections.getActiveSectionId();
  if (activeID === targetID && isSectionActuallyVisible(targetID)) {
    writeStoredSection(targetID);
    return;
  }
  void navigateToSection(targetID, hashID !== targetID).catch((err) => {
    console.error('[SectionManager] URL sync failed', err);
  });
};

window.addEventListener('hashchange', syncSectionFromURL);
window.addEventListener('pageshow', syncSectionFromURL);
window.addEventListener('focus', syncSectionFromURL);
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) syncSectionFromURL();
});

const initialHashSection = readHashSection();
if (initialHashSection) {
  syncSectionFromURL();
} else {
  syncSectionFromURL();
}
