import { VisualizationControl } from "../../util/ui";

export class SettingsVisualization {
  constructor(_container: HTMLElement) {}
  dispose() {}
  setVisible(_visible: boolean) {}
}

export function mountSettings(container: HTMLElement): VisualizationControl {
    const viz = new SettingsVisualization(container);
    return {
        dispose: () => viz.dispose(),
        setVisible: (v) => viz.setVisible(v)
    };
}