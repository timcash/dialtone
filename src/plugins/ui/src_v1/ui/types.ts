export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

// Exactly one underlay per section.
export type CanonicalSectionPrimaryOverlayKind =
  | 'three'
  | 'table'
  | 'terminal'
  | 'docs'
  | 'camera'
  | 'settings';
export type SectionPrimaryOverlayKind =
  | CanonicalSectionPrimaryOverlayKind
  | 'stage'
  | 'xterm'
  | 'video'
  | 'button-list'
  | (string & {});
// Optional overlays layered above the underlay.
export type OverlayKind = 'menu' | 'mode-form' | 'status-bar' | 'chatlog' | 'legend' | SectionPrimaryOverlayKind;

export interface SectionOverlayConfig {
  // Underlay selector and kind (one per section).
  primaryKind: SectionPrimaryOverlayKind;
  primary: string;
  // Form overlay selector.
  modeForm?: string;
  form?: string; // modern alias for modeForm
  // Header overlay selector (legacy name kept for compatibility).
  legend?: string;
  // Optional stream/log overlays.
  chatlog?: string;
  statusBar?: string;
}

export interface HeaderConfig {
  visible?: boolean;
  menuVisible?: boolean;
  title?: string;
}

export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  // Canonical section name, e.g. "robot-hero-stage".
  canonicalName?: string;
  header?: HeaderConfig;
  overlays?: SectionOverlayConfig;
}

export interface AppOptions {
  title?: string;
  debug?: boolean;
  pwa?: boolean | PWAOptions;
}

export interface PWAOptions {
  enabled?: boolean;
  serviceWorkerPath?: string;
  registerOnLoad?: boolean;
  disableInDev?: boolean;
  log?: boolean;
}

const PRIMARY_KIND_ALIASES: Record<string, CanonicalSectionPrimaryOverlayKind> = {
  three: 'three',
  stage: 'three',
  table: 'table',
  terminal: 'terminal',
  xterm: 'terminal',
  docs: 'docs',
  camera: 'camera',
  video: 'camera',
  settings: 'settings',
  'button-list': 'settings',
};

export function normalizePrimaryOverlayKind(kind: string): CanonicalSectionPrimaryOverlayKind | string {
  const normalized = kind.trim().toLowerCase();
  return PRIMARY_KIND_ALIASES[normalized] ?? normalized;
}

export function primaryOverlaySuffixes(kind: string): string[] {
  const canonical = normalizePrimaryOverlayKind(kind);
  switch (canonical) {
    case 'three':
      return ['three', 'stage'];
    case 'table':
      return ['table', 'grid'];
    case 'terminal':
      return ['terminal', 'xterm', 'log'];
    case 'docs':
      return ['docs', 'notes'];
    case 'camera':
      return ['camera', 'video'];
    case 'settings':
      return ['settings', 'button-list'];
    default:
      return [String(canonical)];
  }
}
