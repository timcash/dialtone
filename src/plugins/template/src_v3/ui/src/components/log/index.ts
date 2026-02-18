import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

export function mountLog(container: HTMLElement): VisualizationControl {
  const terminal = container.querySelector("[aria-label='Log Terminal']") as HTMLElement | null;
  const input = container.querySelector("input[aria-label='Log Input']") as HTMLInputElement | null;
  if (!terminal || !input) {
    throw new Error('log terminal not found');
  }

  const term = new Terminal({
    cursorBlink: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace',
    fontSize: 14,
    theme: {
      background: '#000000',
      foreground: '#d1d5db',
      cursor: '#93c5fd',
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminal);
  fit.fit();

  term.writeln('[T+0000] [LOG] tailing /api/test-log');

  const onResize = () => fit.fit();
  window.addEventListener('resize', onResize);

  let logOffset = 0;
  let pollTimer = 0;
  let disposed = false;

  const appendLines = (lines: string[]) => {
    for (const line of lines) {
      term.writeln(line);
    }
  };

  const poll = async () => {
    if (disposed) return;
    try {
      const res = await fetch(`/api/test-log?offset=${logOffset}`);
      if (!res.ok) {
        term.writeln(`[LOG] tail error: ${res.status}`);
      } else {
        const data = (await res.json()) as { offset?: number; lines?: string[] };
        logOffset = Number.isFinite(data.offset) ? Number(data.offset) : logOffset;
        appendLines(Array.isArray(data.lines) ? data.lines : []);
      }
    } catch (error) {
      term.writeln(`[LOG] tail error: ${String(error)}`);
    } finally {
      if (!disposed) {
        pollTimer = window.setTimeout(() => {
          void poll();
        }, 700);
      }
    }
  };

  const onKeyDown = (ev: KeyboardEvent) => {
    if (ev.key !== 'Enter') return;
    const cmd = input.value.trim();
    terminal.setAttribute('data-last-command', cmd);
    if (cmd.length > 0) {
      term.writeln(`[T+0000] USER> ${cmd}`);
    } else {
      term.writeln('[T+0000] USER>');
    }
    input.value = '';
  };
  input.addEventListener('keydown', onKeyDown);

  void poll();

  return {
    dispose: () => {
      disposed = true;
      if (pollTimer) {
        window.clearTimeout(pollTimer);
      }
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
