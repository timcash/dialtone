import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

export function mountXterm(container: HTMLElement): VisualizationControl {
  const terminal = container.querySelector("[aria-label='Xterm Terminal']") as HTMLElement | null;
  if (!terminal) {
    throw new Error('xterm terminal not found');
  }

  let timer = window.setInterval(() => {
    const stamp = new Date().toISOString().split('T')[1].slice(0, 8);
    terminal.textContent = `dialtone@template:~$ heartbeat ${stamp}`;
  }, 1000);

  return {
    dispose: () => {
      clearInterval(timer);
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        terminal.setAttribute('data-ready', 'true');
      }
    },
  };
}
