import { setupApp } from '@ui/ui';
import './style.css';
import { initConnection } from './data/connection';
import { logError, logInfo } from './data/logging';
import { handleButtonKey } from './buttons';
import { ROBOT_SECTION_HASH_ALIASES, ROBOT_SECTION_IDS, ROBOT_SECTION_ORDER, type RobotSectionID } from './section_ids';

declare const APP_VERSION: string;

const { sections, menu } = setupApp({ title: 'dialtone.robot', debug: true });

const isLocalDevHost = ['127.0.0.1', 'localhost'].includes(window.location.hostname);
const normalizeVersion = (v: string) => v.replace(/^v/i, '').trim();

type RobotUpdateStatus = {
  currentVersion: string;
  currentNorm: string;
  latestVersion: string;
  latestNorm: string;
  available: boolean;
  checkedAt: string;
};

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    // Aggressively unregister any service workers to prevent stale app shell
    void navigator.serviceWorker.getRegistrations().then((regs) => {
        regs.forEach((reg) => {
            logInfo('ui/main', `[SW] Unregistering stale worker: ${reg.scope}`);
            void reg.unregister();
        });
    });
    
    // Clear caches that might be holding stale assets
    if ('caches' in window) {
        void caches.keys().then((keys) => {
            keys.forEach((key) => {
            if (key.includes('robot') || key.includes('dialtone') || key.includes('workbox')) {
                logInfo('ui/main', `[Cache] Deleting stale cache: ${key}`);
                void caches.delete(key);
            }
            });
        });
    }
  });
}

// Display version
const versionEl = document.getElementById('app-version');
const currentAppVersion = String(APP_VERSION ?? '').trim();
if (versionEl) {
  const stamp = isLocalDevHost ? ` (dev-${new Date().toLocaleTimeString()})` : '';
  const shown = currentAppVersion || 'dev';
  versionEl.textContent = `v${shown}${stamp}`;
}
(window as any).__robotCurrentVersion = currentAppVersion;

const setMenuUpdateState = (available: boolean) => {
  document.body.setAttribute('data-update-available', available ? 'true' : 'false');
};

const broadcastUpdateStatus = (latestVersion: string, available: boolean) => {
  const status: RobotUpdateStatus = {
    currentVersion: currentAppVersion || 'dev',
    currentNorm: normalizeVersion(currentAppVersion || 'dev'),
    latestVersion: latestVersion || currentAppVersion || 'dev',
    latestNorm: normalizeVersion(latestVersion || currentAppVersion || 'dev'),
    available,
    checkedAt: new Date().toISOString(),
  };
  (window as any).__robotUpdateStatus = status;
  window.dispatchEvent(new CustomEvent('robot-update-status', { detail: status }));
};

const reloadForUpdate = () => {
  const url = new URL(window.location.href);
  url.searchParams.set('refresh', Date.now().toString());
  window.location.replace(url.toString());
};
(window as any).robotReloadForUpdate = reloadForUpdate;

// Initialize Connection (NATS + Polling)
initConnection();

const checkForUpdate = async () => {
  try {
    const res = await fetch('/api/init', { cache: 'no-store' });
    if (!res.ok) return;
    const data = await res.json();
    const nextVersion = String(data.version ?? '').trim();
    const nextNorm = normalizeVersion(nextVersion);
    const currentNorm = normalizeVersion(String(currentAppVersion ?? '').trim());
    const available =
      !!nextNorm &&
      !/^dev$/i.test(nextNorm) &&
      !/^dev$/i.test(currentNorm) &&
      nextNorm !== currentNorm;
    setMenuUpdateState(available);
    broadcastUpdateStatus(nextVersion, available);
  } catch (err) {
    // Ignore offline errors
  }
};

// Check for updates on load and periodically
checkForUpdate();
setInterval(checkForUpdate, 60000);

sections.register(ROBOT_SECTION_IDS.hero, {
  containerId: ROBOT_SECTION_IDS.hero,
  canonicalName: ROBOT_SECTION_IDS.hero,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.hero, 'loading hero ...');
    const { mountHero } = await import('./components/hero/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.hero);
    if (!container) throw new Error('hero container not found');
    return mountHero(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot' },
  overlays: {
    primaryKind: 'stage',
    primary: '.hero-stage',
    form: '.mode-form',
    legend: '.hero-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.docs, {
  containerId: ROBOT_SECTION_IDS.docs,
  canonicalName: ROBOT_SECTION_IDS.docs,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.docs, 'loading documentation ...');
    const { mountDocs } = await import('./components/docs/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.docs);
    if (!container) throw new Error('docs container not found');
    return mountDocs(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Docs' },
  overlays: {
    primaryKind: 'docs',
    primary: '.docs-primary',
    form: '.docs-thumb',
    legend: '.docs-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.settings, {
  containerId: ROBOT_SECTION_IDS.settings,
  canonicalName: ROBOT_SECTION_IDS.settings,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.settings, 'loading settings ...');
    const { mountSettings } = await import('./components/settings/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.settings);
    if (!container) throw new Error('settings container not found');
    return mountSettings(container);
  },
  header: { visible: false, menuVisible: true, title: 'Settings' },
  overlays: {
    primaryKind: 'button-list',
    primary: '.button-list',
    legend: '.settings-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.table, {
  containerId: ROBOT_SECTION_IDS.table,
  canonicalName: ROBOT_SECTION_IDS.table,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.table, 'loading telemetry ...');
    const { mountTable } = await import('./components/table/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.table);
    if (!container) throw new Error('table container not found');
    return mountTable(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Telemetry' },
  overlays: {
    primaryKind: 'table',
    primary: '.table-wrapper', // Updated to wrapper
    form: '.mode-form',
    legend: '.telemetry-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.steeringSettings, {
  containerId: ROBOT_SECTION_IDS.steeringSettings,
  canonicalName: ROBOT_SECTION_IDS.steeringSettings,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.steeringSettings, 'loading steering settings ...');
    const { mountSteeringSettings } = await import('./components/steering_settings/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.steeringSettings);
    if (!container) throw new Error('steering settings container not found');
    return mountSteeringSettings(container);
  },
  header: { visible: false, menuVisible: true, title: 'Steering Settings' },
  overlays: {
    primaryKind: 'table',
    primary: '.table-wrapper',
    form: '.mode-form',
  },
});

sections.register(ROBOT_SECTION_IDS.keyParams, {
  containerId: ROBOT_SECTION_IDS.keyParams,
  canonicalName: ROBOT_SECTION_IDS.keyParams,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.keyParams, 'loading key params ...');
    const { mountKeyParams } = await import('./components/key_params/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.keyParams);
    if (!container) throw new Error('key params container not found');
    return mountKeyParams(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Key Params' },
  overlays: {
    primaryKind: 'table',
    primary: '.table-wrapper',
    legend: '.key-params-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.three, {
  containerId: ROBOT_SECTION_IDS.three,
  canonicalName: ROBOT_SECTION_IDS.three,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.three, 'loading 3d environment ...');
    const container = document.getElementById(ROBOT_SECTION_IDS.three);
    if (!container) throw new Error('three container not found');

    // Apply chatlog setting on load
    const chatlog = container.querySelector('.three-chatlog') as HTMLElement;
    if (chatlog) {
      chatlog.hidden = localStorage.getItem('robot.chatlog.enabled') !== 'true';
    }

    try {
      const { mountThree } = await import('./components/three/index');
      return mountThree(container);
    } catch (err) {
      logError('ui/main', '[Three] mount failed; showing fallback', err);
      const fallbackClass = 'section-load-fallback';
      const fallbackAria = 'Three Unavailable';
      let fallback = container.querySelector(`.${fallbackClass}`) as HTMLDivElement | null;
      if (!fallback) {
        fallback = document.createElement('div');
        fallback.className = fallbackClass;
        fallback.setAttribute('aria-label', fallbackAria);
        container.appendChild(fallback);
      }
      fallback.textContent = '3D view unavailable on this browser session. Check WebGL/GPU and refresh.';
      fallback.hidden = false;
      return {
        dispose: () => {},
        setVisible: (visible: boolean) => {
          if (fallback) fallback.hidden = !visible;
        },
      };
    }
  },
  header: { visible: false, menuVisible: true, title: 'Robot 3D' },
  overlays: {
    primaryKind: 'stage',
    primary: '.three-stage',
    form: '.mode-form',
    legend: '.three-legend',
    chatlog: '.three-chatlog', // Added chatlog overlay
  },
});

sections.register(ROBOT_SECTION_IDS.xterm, {
  containerId: ROBOT_SECTION_IDS.xterm,
  canonicalName: ROBOT_SECTION_IDS.xterm,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.xterm, 'loading terminal ...');
    const { mountXterm } = await import('./components/xterm/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.xterm);
    if (!container) throw new Error('xterm container not found');
    return mountXterm(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Terminal' },
  overlays: {
    primaryKind: 'xterm',
    primary: '.xterm-primary',
    form: '.mode-form',
    legend: '.xterm-legend',
  },
});

sections.register(ROBOT_SECTION_IDS.video, {
  containerId: ROBOT_SECTION_IDS.video,
  canonicalName: ROBOT_SECTION_IDS.video,
  load: async () => {
    sections.setLoadingMessage(ROBOT_SECTION_IDS.video, 'loading camera stream ...');
    const { mountVideo } = await import('./components/video/index');
    const container = document.getElementById(ROBOT_SECTION_IDS.video);
    if (!container) throw new Error('video container not found');
    return mountVideo(container);
  },
  header: { visible: false, menuVisible: true, title: 'Robot Camera' },
  overlays: {
    primaryKind: 'video',
    primary: '.video-stage',
    form: '.mode-form',
    legend: '.video-legend',
  },
});

menu.addButton('Hero', 'Navigate Hero', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.hero);
});
menu.addButton('Docs', 'Navigate Docs', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.docs);
});
menu.addButton('Telemetry', 'Navigate Telemetry', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.table);
});
menu.addButton('Steering', 'Navigate Steering Settings', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.steeringSettings);
});
menu.addButton('Key Params', 'Navigate Key Params', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.keyParams);
});
menu.addButton('Three', 'Navigate Three', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.three);
});
menu.addButton('Terminal', 'Navigate Terminal', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.xterm);
});
menu.addButton('Camera', 'Navigate Camera', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.video);
});
menu.addButton('Settings', 'Navigate Settings', () => {
  void sections.navigateTo(ROBOT_SECTION_IDS.settings);
});

const sectionOrder = ROBOT_SECTION_ORDER;
const sectionSet = new Set(sectionOrder);
const defaultSection = sectionOrder[0];

const syncSectionFromURL = () => {
  const hashId = window.location.hash.slice(1).trim().toLowerCase();
  const fromAlias = ROBOT_SECTION_HASH_ALIASES[hashId];
  const targetId = fromAlias && sectionSet.has(fromAlias) ? fromAlias : defaultSection;
  const activeId = sections.getActiveSectionId();
  if (activeId === targetId) return;
  void sections.navigateTo(targetId, { updateHash: hashId !== targetId }).catch((err: unknown) => {
    logError('ui/main', '[SectionManager] URL sync failed', err);
  });
};

window.addEventListener('hashchange', syncSectionFromURL);
window.addEventListener('pageshow', syncSectionFromURL);
window.addEventListener('focus', syncSectionFromURL);
document.addEventListener('visibilitychange', () => {
  if (!document.hidden) syncSectionFromURL();
});

window.addEventListener('keydown', (event) => {
  if (event.defaultPrevented) return;
  const target = event.target as HTMLElement | null;
  if (target && ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)) return;

  const active = sections.getActiveSectionId() ?? defaultSection;
  
  // Handle Number Keys 1-9
  if (event.key >= '1' && event.key <= '9') {
    event.preventDefault();
    handleButtonKey(active, parseInt(event.key) - 1);
    return;
  }

  const idx = sectionOrder.indexOf(active as RobotSectionID);
  if (idx < 0) return;

  if (event.key === 'ArrowDown' || event.key === 'PageDown' || event.key.toLowerCase() === 'j') {
    const next = sectionOrder[Math.min(sectionOrder.length - 1, idx + 1)];
    if (next !== active) {
      event.preventDefault();
      void sections.navigateTo(next);
    }
    return;
  }
  if (event.key === 'ArrowUp' || event.key === 'PageUp' || event.key.toLowerCase() === 'k') {
    const prev = sectionOrder[Math.max(0, idx - 1)];
    if (prev !== active) {
      event.preventDefault();
      void sections.navigateTo(prev);
    }
    return;
  }
  if (event.key.toLowerCase() === 'm') {
    event.preventDefault();
    const globalMenu = document.querySelector("button[aria-label='Toggle Global Menu']") as HTMLButtonElement | null;
    globalMenu?.click();
  }
});

let lastWheelTime = 0;
const wheelThrottle = 800; // ms

window.addEventListener('wheel', (event) => {
  const now = Date.now();
  if (now - lastWheelTime < wheelThrottle) return;

  const active = sections.getActiveSectionId() ?? defaultSection;
  const idx = sectionOrder.indexOf(active as RobotSectionID);
  if (idx < 0) return;

  if (Math.abs(event.deltaY) > 20) {
    if (event.deltaY > 0 && idx < sectionOrder.length - 1) {
      lastWheelTime = now;
      void sections.navigateTo(sectionOrder[idx + 1]);
    } else if (event.deltaY < 0 && idx > 0) {
      lastWheelTime = now;
      void sections.navigateTo(sectionOrder[idx - 1]);
    }
  }
}, { passive: true });

syncSectionFromURL();
