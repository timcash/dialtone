import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '@ui/types';
import { addRobotEventListener, getRobotEventHistory, sendCommand, type RobotEvent } from '../../data/connection';
import { logInfo } from '../../data/logging';
import { registerButtons, renderButtons, setMode } from '../../buttons';
import { ROBOT_SECTION_IDS } from '../../section_ids';

type CursorPos = {
  row: number;
  col: number;
};

type LogFilter = 'all' | 'mavlink' | 'camera' | 'command' | 'service' | 'ui' | 'error';

type LogLine = {
  text: string;
  filter: Exclude<LogFilter, 'all' | 'error'> | 'unknown';
  level: 'INFO' | 'WARN' | 'ERROR';
  timestamp: string;
};

const MAX_LINES = 300;

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
    scrollback: 3000,
    theme: {
      background: '#000000',
      foreground: '#cfe3ff',
      cursor: '#cfe3ff',
    },
  });
  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminalEl);

  let disposed = false;
  let paused = false;
  let activeFilter: LogFilter = 'all';
  let cursor: CursorPos = { row: 0, col: 0 };
  let selectionAnchor: CursorPos | null = null;
  let unsubscribeEvents: (() => void) | null = null;
  const logLines: LogLine[] = [];
  let lastStatusText = '';
  let lastCommandAckResult = '';
  let lastErrorLine = '';

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
    for (let row = 0; row < pos.row; row += 1) idx += lineLength(row) + 1;
    return idx + pos.col;
  };

  const applyCursorAttrs = () => {
    terminalEl.setAttribute('data-cursor-row', String(cursor.row));
    terminalEl.setAttribute('data-cursor-col', String(cursor.col));
    terminalEl.setAttribute('data-selecting', selectionAnchor ? 'true' : 'false');
    terminalEl.setAttribute('data-filter', activeFilter);
    terminalEl.setAttribute('data-paused', paused ? 'true' : 'false');
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
    if (dy !== 0 && dx === 0) next.col = cursor.col;
    cursor = clampPos(next);
    if (extendSelection && selectionAnchor) {
      paintSelection();
      return;
    }
    if (selectionAnchor) {
      selectionAnchor = null;
      term.clearSelection();
      terminalEl.setAttribute('data-selecting', 'false');
      setMode(ROBOT_SECTION_IDS.xterm, 'Tail');
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
    terminalEl.setAttribute('data-selecting', 'true');
    paintSelection();
    setMode(ROBOT_SECTION_IDS.xterm, 'Select');
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

  const matchFilter = (line: LogLine) => {
    if (activeFilter === 'all') return true;
    if (activeFilter === 'error') return line.level === 'ERROR';
    return line.filter === activeFilter;
  };

  const syncTerminalAttrs = () => {
    const filtered = logLines.filter(matchFilter);
    const last = filtered[filtered.length - 1];
    const filteredLastError = [...filtered].reverse().find((line) => line.level === 'ERROR');
    terminalEl.setAttribute('data-total-lines', String(filtered.length));
    terminalEl.setAttribute('data-last-log-line', last?.text || '');
    terminalEl.setAttribute('data-last-log-category', last?.filter || '');
    terminalEl.setAttribute('data-last-log-level', last?.level || '');
    terminalEl.setAttribute('data-last-error-line', lastErrorLine || filteredLastError?.text || '');
    terminalEl.setAttribute('data-last-status-text', lastStatusText);
    terminalEl.setAttribute('data-last-command-ack-result', lastCommandAckResult);
    applyCursorAttrs();
  };

  const renderLogs = () => {
    const filtered = logLines.filter(matchFilter);
    term.reset();
    term.writeln('[ROBOT TERM] ready');
    term.writeln('[ROBOT TERM] unified NATS log bus active');
    term.writeln(`[ROBOT TERM] filter=${activeFilter} paused=${paused ? 'true' : 'false'}`);
    filtered.slice(-MAX_LINES).forEach((line) => term.writeln(line.text));
    cursor = clampPos({ row: maxRow(), col: lineLength(maxRow()) });
    paintCursor();
    syncTerminalAttrs();
  };

  const setFilter = (filter: LogFilter) => {
    activeFilter = filter;
    renderLogs();
  };

  const togglePause = () => {
    paused = !paused;
    renderButtons(ROBOT_SECTION_IDS.xterm);
    syncTerminalAttrs();
  };

  const clearLogs = () => {
    logLines.splice(0, logLines.length);
    renderLogs();
  };

  const submitInput = () => {
    const value = inputEl.value.trim();
    if (!value) return;
    const inputAria = inputEl.getAttribute('aria-label') || 'Log Command Input';
    logInfo('ui/xterm', `[TEST_ACTION] input aria=${inputAria} value=${value}`);
    terminalEl.setAttribute('data-last-command', value);
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
    inputEl.value = '';
  };

  const appendEvent = (event: RobotEvent) => {
    const filter = (event.category === 'unknown' ? 'service' : event.category) as LogLine['filter'];
    const line: LogLine = {
      text: event.logLine,
      filter,
      level: event.level,
      timestamp: event.timestamp,
    };
    logLines.push(line);
    if (logLines.length > MAX_LINES * 4) {
      logLines.splice(0, logLines.length - MAX_LINES * 4);
    }
    if (event.subject === 'mavlink.command_ack') {
      const result = String(event.payload?.result || '').trim();
      if (result !== '') {
        lastCommandAckResult = result;
      }
    }
    if (event.subject === 'mavlink.statustext') {
      const text = String(event.payload?.text || '').trim();
      if (text !== '') {
        lastStatusText = text;
      }
    }
    if (event.level === 'ERROR' && line.text.trim() !== '') {
      lastErrorLine = line.text;
    }
    if (!paused) {
      renderLogs();
    } else {
      syncTerminalAttrs();
    }
  };

  const bootstrapHistory = () => {
    logLines.splice(0, logLines.length);
    getRobotEventHistory({ category: 'all' }).forEach((event) => appendEvent(event));
    if (logLines.length === 0) {
      renderLogs();
    }
  };

  registerButtons(ROBOT_SECTION_IDS.xterm, ['Tail', 'Filter', 'Command', 'Select'], {
    Tail: [
      { label: 'Left', action: () => moveCursor(-1, 0, false) },
      { label: 'Right', action: () => moveCursor(1, 0, false) },
      { label: 'Up', action: () => moveCursor(0, -1, false) },
      { label: 'Down', action: () => moveCursor(0, 1, false) },
      { label: 'Home', action: () => moveHome(false) },
      { label: 'End', action: () => moveEnd(false) },
      { label: paused ? 'Resume' : 'Pause', action: () => togglePause(), active: paused },
      { label: 'Copy', action: () => copySelection() },
    ],
    Filter: [
      { label: 'All', action: () => setFilter('all'), active: activeFilter === 'all' },
      { label: 'MAV', action: () => setFilter('mavlink'), active: activeFilter === 'mavlink' },
      { label: 'Cmd', action: () => setFilter('command'), active: activeFilter === 'command' },
      { label: 'UI', action: () => setFilter('ui'), active: activeFilter === 'ui' },
      { label: 'Cam', action: () => setFilter('camera'), active: activeFilter === 'camera' },
      { label: 'Svc', action: () => setFilter('service'), active: activeFilter === 'service' },
      { label: 'Err', action: () => setFilter('error'), active: activeFilter === 'error' },
      { label: 'Clear', action: () => clearLogs() },
    ],
    Command: [
      { label: 'Send', action: () => submitInput() },
      { label: 'Arm', action: () => sendCommand('arm') },
      { label: 'Disarm', action: () => sendCommand('disarm') },
      { label: 'Manual', action: () => sendCommand('mode', 'manual') },
      { label: 'Guided', action: () => sendCommand('mode', 'guided') },
      { label: 'Stop', action: () => sendCommand('stop') },
      { label: 'Tail', action: () => setMode(ROBOT_SECTION_IDS.xterm, 'Tail') },
      { label: 'Filters', action: () => setMode(ROBOT_SECTION_IDS.xterm, 'Filter') },
    ],
    Select: [
      { label: 'Left', action: () => moveCursor(-1, 0, true) },
      { label: 'Right', action: () => moveCursor(1, 0, true) },
      { label: 'Up', action: () => moveCursor(0, -1, true) },
      { label: 'Down', action: () => moveCursor(0, 1, true) },
      { label: 'Start', action: () => startSelection() },
      {
        label: 'Clear', action: () => {
          selectionAnchor = null;
          term.clearSelection();
          terminalEl.setAttribute('data-selecting', 'false');
          paintCursor();
          setMode(ROBOT_SECTION_IDS.xterm, 'Tail');
        },
      },
      { label: 'Copy', action: () => copySelection() },
      {
        label: 'Done', action: () => {
          selectionAnchor = null;
          term.clearSelection();
          terminalEl.setAttribute('data-selecting', 'false');
          paintCursor();
          setMode(ROBOT_SECTION_IDS.xterm, 'Tail');
        },
      },
    ],
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

  unsubscribeEvents = addRobotEventListener((event) => {
    if (disposed) return;
    appendEvent(event);
  }, { replay: false });
  bootstrapHistory();

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
  queueMicrotask(safeFit);

  return {
    dispose: () => {
      disposed = true;
      window.removeEventListener('resize', onResize);
      submitBtn.removeEventListener('click', submitInput);
      controlsEl.removeEventListener('submit', onFormSubmit);
      inputEl.removeEventListener('keydown', onInputKeyDown);
      if (unsubscribeEvents) unsubscribeEvents();
      term.dispose();
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        requestAnimationFrame(() => safeFit());
        inputEl.focus();
        terminalEl.setAttribute('data-ready', 'true');
        controlsEl.setAttribute('data-ready', 'true');
        renderButtons(ROBOT_SECTION_IDS.xterm);
        renderLogs();
      }
    },
  };
}
