export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

export type SectionPrimaryOverlayKind = 'stage' | 'table' | 'xterm' | 'docs' | 'video' | (string & {});
export type OverlayKind = 'menu' | 'mode-form' | 'status-bar' | 'chatlog' | 'legend' | 'thumb' | SectionPrimaryOverlayKind;

export interface SectionOverlayConfig {
  primaryKind: SectionPrimaryOverlayKind;
  primary: string;
  modeForm?: string;
  form?: string; // modern alias for modeForm
  thumb?: string; // deprecated alias for modeForm
  legend?: string;
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
  header?: HeaderConfig;
  overlays?: SectionOverlayConfig;
}

export interface AppOptions {
  title?: string;
  debug?: boolean;
}
