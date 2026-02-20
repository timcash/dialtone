import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '@ui/types';
import { addMavlinkListener, sendCommand } from '../../data/connection';
import { registerButtons, renderButtons, setMode } from '../../buttons';

type CursorPos = {
  row: number;
  col: number;
};

export function mountXterm(container: HTMLElement): VisualizationControl {
  const terminalEl = container.querySelector("[aria-label='Xterm Terminal']") as HTMLElement | null;
  const controlsEl = container.querySelector("[aria-label='Log Mode Form']") as HTMLFormElement | null;
  const inputEl = container.querySelector("input[aria-label='Log Command Input']") as HTMLInputElement | null;
  const submitBtn = container.querySelector("button[aria-label='Log Submit']") as HTMLButtonElement | null;
  
  if (!terminalEl || !controlsEl || !inputEl || !submitBtn) {
    throw new Error('xterm terminal controls not found');
  }

  const term = new Terminal({
    cursorBlink: true,
    convertEol: true,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    fontSize: 14,
    lineHeight: 1.2,
    scrollback: 1500,
    theme: {
      background: '#000000',
      foreground: '#cfe3ff',
      cursor: '#cfe3ff',
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminalEl);
  term.writeln('[ROBOT TERM] ready');
  term.writeln('[ROBOT TERM] listening to mavlink...');

  let disposed = false;
  let cursor: CursorPos = { row: 0, col: 0 };
  let selectionAnchor: CursorPos | null = null;
  let unsubscribeMav: (() => void) | null = null;

  const lineText = (row: number): string => {
    const line = term.buffer.active.getLine(row);
    return line ? line.translateToString(true) : '';
  };

  const lineLength = (row: number): number => lineText(row).length;

  const maxRow = (): number => Math.max(0, term.buffer.active.length - 1);

  const clampPos = (pos: CursorPos): CursorPos => {
    const row = Math.max(0, Math.min(maxRow(), pos.row));
    const col = Math.max(0, Math.min(lineLength(row), pos.col));
    return { row, col };
  };

  const posToLinear = (pos: CursorPos): number => {
    let idx = 0;
    for (let row = 0; row < pos.row; row += 1) {
      idx += lineLength(row) + 1;
    }
    return idx + pos.col;
  };

  const applyCursorAttrs = () => {
    terminalEl.setAttribute('data-cursor-row', String(cursor.row));
    terminalEl.setAttribute('data-cursor-col', String(cursor.col));
    terminalEl.setAttribute('data-selecting', selectionAnchor ? 'true' : 'false');
  };

  const paintCursor = () => {
    term.select(cursor.col, cursor.row, 1);
    applyCursorAttrs();
  };

  const paintSelection = () => {
    if (!selectionAnchor) {
      paintCursor();
      return;
    }
    const a = clampPos(selectionAnchor);
    const b = clampPos(cursor);
    const aIdx = posToLinear(a);
    const bIdx = posToLinear(b);
    const start = aIdx <= bIdx ? a : b;
    const length = Math.max(1, Math.abs(bIdx - aIdx) + 1);
    term.select(start.col, start.row, length);
    applyCursorAttrs();
  };

  const moveCursor = (dx: number, dy: number, extendSelection: boolean) => {
    const next: CursorPos = { row: cursor.row + dy, col: cursor.col + dx };
    if (dy !== 0 && dx === 0) {
      next.col = cursor.col;
    }
    cursor = clampPos(next);
    if (extendSelection && selectionAnchor) {
      paintSelection();
      return;
    }
    // Auto-clear selection if moving without extend
    if (selectionAnchor) {
      selectionAnchor = null;
      term.clearSelection();
      setMode('xterm', 'Cursor'); // Fallback to cursor mode if we were selecting
    }
    paintCursor();
  };

  const moveHome = (extendSelection: boolean) => {
    cursor = clampPos({ row: cursor.row, col: 0 });
    if (extendSelection && selectionAnchor) {
      paintSelection();
      return;
    }
    paintCursor();
  };

  const moveEnd = (extendSelection: boolean) => {
    cursor = clampPos({ row: cursor.row, col: lineLength(cursor.row) });
    if (extendSelection && selectionAnchor) {
      paintSelection();
      return;
    }
    paintCursor();
  };

  const startSelection = () => {
    selectionAnchor = { ...cursor };
    paintSelection();
    setMode('xterm', 'Select');
  };

  const copySelection = async () => {
    const selected = term.getSelection();
    if (!selected) {
      term.writeln('[TERM] COPY> no selection');
      return;
    }
    try {
      if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
        await navigator.clipboard.writeText(selected);
      } else {
        throw new Error('clipboard API unavailable');
      }
      term.writeln(`[TERM] COPY> ${selected.length} chars`);
    } catch {
      term.writeln('[TERM] COPY> clipboard unavailable');
    }
  };

  const submitInput = () => {
    const value = inputEl.value.trim();
    if (!value) return;
    
    term.writeln(`$ ${value}`);
    
    if (value.startsWith('mode ')) {
        const parts = value.split(' ');
        if (parts.length > 1) sendCommand('mode', parts[1]);
    } else if (value === 'arm') {
        sendCommand('arm');
    } else if (value === 'disarm') {
        sendCommand('disarm');
    } else {
        sendCommand(value);
    }

    input.value = '';
    cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
    paintCursor();
  };

  // Register Buttons
  registerButtons('xterm', ['Cursor', 'Select', 'Command'], {
    'Cursor': [
      { label: 'Left', action: () => moveCursor(-1, 0, false) },
      { label: 'Right', action: () => moveCursor(1, 0, false) },
      { label: 'Up', action: () => moveCursor(0, -1, false) },
      { label: 'Down', action: () => moveCursor(0, 1, false) },
      { label: 'Home', action: () => moveHome(false) },
      { label: 'End', action: () => moveEnd(false) },
      { label: 'Select', action: () => startSelection() },
      { label: 'Copy', action: () => copySelection() },
    ],
    'Select': [
      { label: 'Left', action: () => moveCursor(-1, 0, true) },
      { label: 'Right', action: () => moveCursor(1, 0, true) },
      { label: 'Up', action: () => moveCursor(0, -1, true) },
      { label: 'Down', action: () => moveCursor(0, 1, true) },
      { label: 'Start', action: () => startSelection() }, // Restart anchor?
      { label: 'Clear', action: () => {
          selectionAnchor = null;
          term.clearSelection();
          paintCursor();
          setMode('xterm', 'Cursor');
      }},
      { label: 'Copy', action: () => copySelection() },
      { label: 'Done', action: () => {
          selectionAnchor = null;
          term.clearSelection();
          paintCursor();
          setMode('xterm', 'Cursor');
      }},
    ],
    'Command': [
      { label: 'Send', action: () => submitInput() },
      { label: 'Clear', action: () => { inputEl.value = ''; inputEl.focus(); } },
      { label: 'Left', action: () => moveCursor(-1, 0, false) },
      { label: 'Right', action: () => moveCursor(1, 0, false) },
      { label: 'Up', action: () => moveCursor(0, -1, false) },
      { label: 'Down', action: () => moveCursor(0, 1, false) },
      { label: 'Select', action: () => startSelection() },
      { label: 'Copy', action: () => copySelection() },
    ]
  });

  submitBtn.addEventListener('click', submitInput);
  const onFormSubmit = (event: SubmitEvent) => {
    event.preventDefault();
    submitInput();
  };
  controlsEl.addEventListener('submit', onFormSubmit);

  const onInputKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    submitInput();
  };
  inputEl.addEventListener('keydown', onInputKeyDown);

  const subscribeToMavlink = () => {
    if (unsubscribeMav) return;
    unsubscribeMav = addMavlinkListener((data: any) => {
        if (disposed) return;
        
        let msg = '';
        const ts = new Date().toLocaleTimeString();
        
        if (data.text) {
            const sev = data.severity !== undefined ? `[SEV${data.severity}]` : '';
            msg = `[${ts}] ${sev} ${data.text}`;
        } else if (data.command && data.result) {
            msg = `[${ts}] CMD_ACK: cmd=${data.command} res=${data.result}`;
        }

        if (msg) {
            term.writeln(msg);
            if (cursor.row >= maxRow() - 1) {
                cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
                paintCursor();
            }
        }
    });
  };

  subscribeToMavlink();

  const safeFit = () => {
    if (container.hidden) return;
    if (terminalEl.clientWidth <= 0 || terminalEl.clientHeight <= 0) return;
    try {
      fit.fit();
    } catch {
      // xterm fit can throw if called before the section is fully visible.
    }
  };

  const onResize = () => safeFit();
  window.addEventListener('resize', onResize);

  cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
  paintCursor();
  queueMicrotask(safeFit);

  return {
    dispose: () => {
      disposed = true;
      window.removeEventListener('resize', onResize);
      submitBtn.removeEventListener('click', submitInput);
      controlsEl.removeEventListener('submit', onFormSubmit);
      inputEl.removeEventListener('keydown', onInputKeyDown);
      if (unsubscribeMav) unsubscribeMav();
      term.dispose();
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        requestAnimationFrame(() => safeFit());
        inputEl.focus();
        terminalEl.setAttribute('data-ready', 'true');
        controlsEl.setAttribute('data-ready', 'true');
        subscribeToMavlink();
        renderButtons('xterm');
      } else {
        if (unsubscribeMav) {
            unsubscribeMav();
            unsubscribeMav = null;
        }
      }
    },
  };
}
