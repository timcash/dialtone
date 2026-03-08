import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

export type SignalTerminal = {
  setLines: (lines: string[]) => void;
  dispose: () => void;
};

export function mountSignalTerminal(container: HTMLElement): SignalTerminal {
  const term = new Terminal({
    cursorBlink: false,
    convertEol: true,
    disableStdin: true,
    fontFamily:
      "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
    fontSize: 14,
    lineHeight: 1.2,
    scrollback: 1500,
    theme: {
      background: "#000000",
      foreground: "#d8f3ff",
      cursor: "#d8f3ff",
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(container);

  const fitNow = () => {
    try {
      fit.fit();
    } catch {
      // xterm can throw during hidden/layout transitions; ignore and refit later.
    }
  };

  const ro = new ResizeObserver(() => fitNow());
  ro.observe(container);
  window.setTimeout(fitNow, 0);

  return {
    setLines(lines: string[]) {
      term.reset();
      for (const line of lines) {
        term.writeln(`[fixture] ${line}`);
      }
      fitNow();
    },
    dispose() {
      ro.disconnect();
      term.dispose();
    },
  };
}
