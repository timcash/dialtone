export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
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
}

export interface AppOptions {
  title?: string;
  debug?: boolean;
}
