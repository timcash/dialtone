/**
 * Core interfaces for the Dialtone UI library.
 */

// Header configuration for a section
export interface HeaderConfig {
  visible?: boolean;
  title?: string;
  subtitle?: string;
  telemetry?: boolean;
  version?: boolean;
  menuVisible?: boolean;
}

// Interface that all visualization controls must implement
export interface VisualizationControl {
  dispose: () => void;
  setVisible: (visible: boolean) => void;
}

// Configuration for a lazy-loaded section
export interface SectionConfig {
  containerId: string;
  load: () => Promise<VisualizationControl>;
  header?: HeaderConfig;
}

// Standard options for initializing the app
export interface AppOptions {
  title: string;
  debug?: boolean;
}