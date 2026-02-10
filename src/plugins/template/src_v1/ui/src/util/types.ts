// Header configuration for a section
export interface HeaderConfig {
  visible?: boolean;
  title?: string;
  subtitle?: string;
  telemetry?: boolean;
  version?: boolean;
}

// Interface that all visualization controls must implement
export interface SectionComponent {
  mount: () => Promise<void>;
  unmount: () => void;
  setVisible: (visible: boolean) => void;
}

// Configuration for a section
export interface SectionConfig {
  component: new (container: HTMLElement) => SectionComponent;
  header?: HeaderConfig;
}