import type { PWAOptions } from './types';

const DEFAULT_PWA_OPTIONS: Required<PWAOptions> = {
  enabled: false,
  serviceWorkerPath: '/sw.js',
  registerOnLoad: true,
  disableInDev: true,
  log: true,
};

function resolvePWAOptions(raw?: boolean | PWAOptions): Required<PWAOptions> {
  if (typeof raw === 'boolean') {
    return { ...DEFAULT_PWA_OPTIONS, enabled: raw };
  }
  if (!raw) {
    return { ...DEFAULT_PWA_OPTIONS };
  }
  return {
    ...DEFAULT_PWA_OPTIONS,
    ...raw,
    enabled: raw.enabled ?? true,
  };
}

export function setupPWA(raw?: boolean | PWAOptions): void {
  const opts = resolvePWAOptions(raw);
  if (!opts.enabled) return;
  if (!('serviceWorker' in navigator)) return;
  if (opts.disableInDev && typeof import.meta !== 'undefined' && (import.meta as any).env?.DEV) return;

  const doRegister = () => {
    void navigator.serviceWorker.register(opts.serviceWorkerPath).catch((err) => {
      if (opts.log) console.error('[ui:pwa] failed to register service worker', err);
    });
  };

  if (opts.registerOnLoad && document.readyState !== 'complete') {
    window.addEventListener('load', doRegister, { once: true });
    return;
  }
  doRegister();
}

