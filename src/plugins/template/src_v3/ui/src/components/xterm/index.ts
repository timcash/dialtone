import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

export function mountXterm(container: HTMLElement): VisualizationControl {
  const terminal = container.querySelector("[aria-label='Xterm Terminal']") as HTMLElement | null;
  if (!terminal) {
    throw new Error('xterm terminal not found');
  }

  return {
    dispose: () => {},
    setVisible: (visible: boolean) => {
      if (visible) {
        terminal.setAttribute('data-ready', 'true');
      }
    },
  };
}
