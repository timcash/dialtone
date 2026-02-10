import { VisualizationControl } from "../../util/ui";

export class TableVisualization {
  constructor(_container: HTMLElement) {}
  dispose() {}
  setVisible(_visible: boolean) {}
}

export function mountTable(container: HTMLElement): VisualizationControl {
    const viz = new TableVisualization(container);
    return {
        dispose: () => viz.dispose(),
        setVisible: (v) => viz.setVisible(v)
    };
}
