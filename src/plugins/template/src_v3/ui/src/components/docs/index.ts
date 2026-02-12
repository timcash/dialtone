import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

export function mountDocs(_container: HTMLElement): VisualizationControl {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  };
}
