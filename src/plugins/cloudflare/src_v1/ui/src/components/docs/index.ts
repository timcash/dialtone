import { VisualizationControl } from '@ui/types';

export function mountDocs(_container: HTMLElement): VisualizationControl {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  };
}
