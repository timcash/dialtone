import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export function mountXterm(container: HTMLElement): VisualizationControl {
  const terminal = container.querySelector("[aria-label='Xterm Terminal']") as HTMLElement | null;
  const input = container.querySelector("input[aria-label='Xterm Input']") as HTMLInputElement | null;
  if (!terminal || !input) {
    throw new Error('xterm terminal not found');
  }

  const term = new Terminal({
    cursorBlink: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace',
    fontSize: 14,
    theme: {
      background: '#000000',
      foreground: '#ffffff',
      cursor: '#ffffff',
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminal);
  fit.fit();

  term.writeln('dialtone@cloudflare:~$ booting...');
  term.writeln('dialtone@cloudflare:~$ ready');

  const onResize = () => fit.fit();
  window.addEventListener('resize', onResize);

  const onKeyDown = (ev: KeyboardEvent) => {
    if (ev.key !== 'Enter') return;
    const cmd = input.value.trim();
    terminal.setAttribute('data-last-command', cmd);
    if (cmd.length > 0) {
      term.writeln(`$ ${cmd}`);
      term.writeln(`executed: ${cmd}`);
    } else {
      term.writeln('$');
    }
    input.value = '';
  };
  input.addEventListener('keydown', onKeyDown);

  return {
    dispose: () => {
      input.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('resize', onResize);
      term.dispose();
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        fit.fit();
        input.focus();
        terminal.setAttribute('data-ready', 'true');
      }
    },
  };
}
