export interface SectionComponent {
  mount: () => Promise<void>;
  unmount: () => void;
  setVisible: (visible: boolean) => void;
}

export interface HeaderConfig {
  visible?: boolean;
}

export interface MenuConfig {
  visible?: boolean;
}

export interface SectionConfig {
  component: new (container: HTMLElement) => SectionComponent;
  header?: HeaderConfig;
  menu?: MenuConfig;
}
