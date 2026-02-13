import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

class TableControl implements VisualizationControl {
  constructor(private container: HTMLElement) {}

  dispose(): void {}

  setVisible(visible: boolean): void {
    const table = this.container.querySelector("table[aria-label='Template Table']") as HTMLElement | null;
    if (!table) return;
    if (visible) {
      table.setAttribute('data-ready', 'true');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
