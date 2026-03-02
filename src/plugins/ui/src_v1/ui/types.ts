export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

// Exactly one underlay per section.
export type SectionPrimaryOverlayKind = 'stage' | 'table' | 'xterm' | 'docs' | 'video' | 'button-list' | (string & {});
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
}
