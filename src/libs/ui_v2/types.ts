export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

export type SectionPrimaryOverlayKind = 'stage' | 'table' | 'xterm' | 'docs' | (string & {});
export type OverlayKind = 'menu' | 'thumb' | 'legend' | SectionPrimaryOverlayKind;

export interface SectionOverlayConfig {
  primaryKind: SectionPrimaryOverlayKind;
  primary: string;
  thumb: string;
  legend: string;
  chatlog?: string;
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
