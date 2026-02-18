import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type LogThumbMode = 'cursor' | 'select' | 'command';

type CursorPos = {
  row: number;
  col: number;
};

type ThumbAction = {
  label: string;
  aria: string;
  run: () => void | Promise<void>;
};

const modeOrder: LogThumbMode[] = ['cursor', 'select', 'command'];
const modeLabel: Record<LogThumbMode, string> = {
  cursor: 'Mode: Cursor',
  select: 'Mode: Select',
  command: 'Mode: Command',
};

const LOG_PREFIX = '[LOGS LOG]';

export function mountLog(container: HTMLElement): VisualizationControl {
  const terminalEl = container.querySelector("[aria-label='Log Terminal']") as HTMLElement | null;
  const controlsEl = container.querySelector("[aria-label='Log Mode Form']") as HTMLFormElement | null;
  const inputEl = container.querySelector("input[aria-label='Log Command Input']") as HTMLInputElement | null;
  const submitBtn = container.querySelector("button[aria-label='Log Submit']") as HTMLButtonElement | null;
  const modeBtn = container.querySelector("button[aria-label='Log Mode']") as HTMLButtonElement | null;
  const thumbButtons = Array.from(container.querySelectorAll("button[aria-label^='Log Thumb']")) as HTMLButtonElement[];
  if (!terminalEl || !controlsEl || !inputEl || !submitBtn || !modeBtn || thumbButtons.length !== 8) {
    throw new Error('log terminal controls not found');
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
  term.writeln(`${LOG_PREFIX} ready`);
  term.writeln(`${LOG_PREFIX} mode-form enabled: cursor/select/command`);
  term.writeln(`${LOG_PREFIX} tailing /api/test-log ...`);

  let disposed = false;
  let offset = 0;
  let polling = false;
  let backendState: 'unknown' | 'waiting' | 'connected' = 'unknown';
  let mode: LogThumbMode = 'cursor';
  let cursor: CursorPos = { row: 0, col: 0 };
  let selectionAnchor: CursorPos | null = null;

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
    terminalEl.setAttribute('data-thumb-mode', mode);
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

  const clearSelection = () => {
    selectionAnchor = null;
    term.clearSelection();
    paintCursor();
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
    if (mode !== 'select') {
      selectionAnchor = null;
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
    mode = 'select';
    paintSelection();
    renderThumbs();
  };

  const copySelection = async () => {
    const selected = term.getSelection();
    if (!selected) {
      term.writeln(`${LOG_PREFIX} COPY> no selection`);
      return;
    }
    try {
      if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
        await navigator.clipboard.writeText(selected);
      } else {
        throw new Error('clipboard API unavailable');
      }
      term.writeln(`${LOG_PREFIX} COPY> ${selected.length} chars`);
    } catch {
      term.writeln(`${LOG_PREFIX} COPY> clipboard unavailable`);
    }
  };

  const cycleMode = () => {
    const idx = modeOrder.indexOf(mode);
    mode = modeOrder[(idx + 1) % modeOrder.length];
    if (mode !== 'select') {
      selectionAnchor = null;
      term.clearSelection();
    }
    renderThumbs();
    paintCursor();
  };

  const submitInput = () => {
    const value = inputEl.value.trim();
    const stamp = new Date().toISOString().replace('T', ' ').slice(0, 19);
    if (!value) {
      term.writeln(`[${stamp}] USER> (empty)`);
    } else {
      term.writeln(`[${stamp}] USER> ${value}`);
    }
    inputEl.value = '';
    cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
    paintCursor();
  };

  const renderThumbs = () => {
    const actionsByMode: Record<LogThumbMode, ThumbAction[]> = {
      cursor: [
        { label: 'Left', aria: 'Log Left', run: () => moveCursor(-1, 0, false) },
        { label: 'Right', aria: 'Log Right', run: () => moveCursor(1, 0, false) },
        { label: 'Up', aria: 'Log Up', run: () => moveCursor(0, -1, false) },
        { label: 'Down', aria: 'Log Down', run: () => moveCursor(0, 1, false) },
        { label: 'Home', aria: 'Log Home', run: () => moveHome(false) },
        { label: 'End', aria: 'Log End', run: () => moveEnd(false) },
        { label: 'Select', aria: 'Log Select', run: startSelection },
        { label: 'Copy', aria: 'Log Copy', run: () => void copySelection() },
      ],
      select: [
        { label: 'Left', aria: 'Log Left', run: () => moveCursor(-1, 0, true) },
        { label: 'Right', aria: 'Log Right', run: () => moveCursor(1, 0, true) },
        { label: 'Up', aria: 'Log Up', run: () => moveCursor(0, -1, true) },
        { label: 'Down', aria: 'Log Down', run: () => moveCursor(0, 1, true) },
        { label: 'Start', aria: 'Log Start', run: startSelection },
        {
          label: 'Clear',
          aria: 'Log Clear Selection',
          run: () => {
            mode = 'cursor';
            clearSelection();
            renderThumbs();
          },
        },
        { label: 'Copy', aria: 'Log Copy', run: () => void copySelection() },
        {
          label: 'Done',
          aria: 'Log Select Done',
          run: () => {
            mode = 'cursor';
            clearSelection();
            renderThumbs();
          },
        },
      ],
      command: [
        { label: 'Send', aria: 'Log Send', run: submitInput },
        {
          label: 'Clear',
          aria: 'Log Clear Input',
          run: () => {
            inputEl.value = '';
            inputEl.focus();
          },
        },
        { label: 'Left', aria: 'Log Left', run: () => moveCursor(-1, 0, false) },
        { label: 'Right', aria: 'Log Right', run: () => moveCursor(1, 0, false) },
        { label: 'Up', aria: 'Log Up', run: () => moveCursor(0, -1, false) },
        { label: 'Down', aria: 'Log Down', run: () => moveCursor(0, 1, false) },
        { label: 'Select', aria: 'Log Select', run: startSelection },
        { label: 'Copy', aria: 'Log Copy', run: () => void copySelection() },
      ],
    };

    const actions = actionsByMode[mode];
    for (let i = 0; i < thumbButtons.length; i += 1) {
      const action = actions[i];
      const button = thumbButtons[i];
      button.textContent = `${i + 1}:${action.label}`;
      button.setAttribute('aria-label', action.aria);
      button.onclick = () => {
        void action.run();
      };
    }
    modeBtn.textContent = `9:${modeLabel[mode]}`;
    modeBtn.setAttribute('data-mode', mode);
  };

  modeBtn.addEventListener('click', cycleMode);
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

  const poll = async () => {
    if (disposed || polling) return;
    polling = true;
    const wasAtBottom = cursor.row >= maxRow() - 1;
    try {
      const res = await fetch(`/api/test-log?offset=${offset}`, { method: 'GET' });
      if (!res.ok) {
        if (backendState !== 'waiting') {
          term.writeln(`${LOG_PREFIX} waiting for backend /api/test-log ...`);
          backendState = 'waiting';
        }
        return;
      }
      if (backendState !== 'connected') {
        term.writeln(`${LOG_PREFIX} connected to /api/test-log`);
        backendState = 'connected';
      }
      const payload = (await res.json()) as { offset?: number; lines?: string[] };
      if (typeof payload.offset === 'number' && Number.isFinite(payload.offset)) {
        offset = Math.max(0, Math.floor(payload.offset));
      }
      const lines = Array.isArray(payload.lines) ? payload.lines : [];
      for (const line of lines) {
        term.writeln(line);
      }
      if (wasAtBottom && !selectionAnchor) {
        cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
        paintCursor();
      }
    } catch {
      if (backendState !== 'waiting') {
        term.writeln(`${LOG_PREFIX} waiting for backend /api/test-log ...`);
        backendState = 'waiting';
      }
    } finally {
      polling = false;
    }
  };

  const intervalID = window.setInterval(() => {
    void poll();
  }, 500);
  void poll();

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

  renderThumbs();
  cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
  paintCursor();
  queueMicrotask(safeFit);

  return {
    dispose: () => {
      disposed = true;
      window.clearInterval(intervalID);
      window.removeEventListener('resize', onResize);
      modeBtn.removeEventListener('click', cycleMode);
      submitBtn.removeEventListener('click', submitInput);
      controlsEl.removeEventListener('submit', onFormSubmit);
      inputEl.removeEventListener('keydown', onInputKeyDown);
      for (const button of thumbButtons) {
        button.onclick = null;
      }
      term.dispose();
    },
    setVisible: (visible: boolean) => {
      if (!visible) return;
      requestAnimationFrame(() => safeFit());
      inputEl.focus();
      terminalEl.setAttribute('data-ready', 'true');
      controlsEl.setAttribute('data-ready', 'true');
    },
  };
}
