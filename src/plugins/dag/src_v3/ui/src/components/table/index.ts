import { VisualizationControl } from '../../../../../../../plugins/ui/types';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

type TableMode = 'browse' | 'filter';

class TableControl implements VisualizationControl {
  private allRows: TableRow[] = [];
  private visibleRows: TableRow[] = [];
  private selectedIndex = 0;
  private mode: TableMode = 'browse';
  private loadPromise: Promise<void> | null = null;
  private lastLoadFailed = false;
  private retryBlockedUntil = 0;
  private readonly apiReadyStorageKey = 'dag.src_v3.api_ready';

  private form: HTMLFormElement | null = null;
  private modeButton: HTMLButtonElement | null = null;
  private actionButtons: HTMLButtonElement[] = [];
  private input: HTMLInputElement | null = null;
  private submitButton: HTMLButtonElement | null = null;

  constructor(private container: HTMLElement) {
    this.bindModeForm();
  }

  dispose(): void {
    // no-op
  }

  private getElements() {
    const table = this.container.querySelector("table[aria-label='DAG Table']") as HTMLTableElement | null;
    if (!table) return { table: null, tbody: null as HTMLTableSectionElement | null };
    const tbody = table.querySelector('tbody');
    return { table, tbody };
  }

  private escapeHTML(value: string): string {
    return value
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;');
  }

  private renderRows(rows: TableRow[]): void {
    const { tbody } = this.getElements();
    if (!tbody) return;
    tbody.innerHTML = rows
      .map((r, idx) => {
        const selected = idx === this.selectedIndex ? 'true' : 'false';
        return `<tr data-selected="${selected}"><td>${this.escapeHTML(r.key)}</td><td>${this.escapeHTML(r.value)}</td><td>${this.escapeHTML(r.status)}</td></tr>`;
      })
      .join('');
  }

  private renderVisibleRows(): void {
    const { table, tbody } = this.getElements();
    if (!table || !tbody) return;
    this.selectedIndex = Math.max(0, Math.min(this.selectedIndex, Math.max(0, this.visibleRows.length - 1)));
    table.setAttribute('data-total-rows', String(this.allRows.length));
    table.setAttribute('data-visible-rows', String(this.visibleRows.length));
    this.renderRows(this.visibleRows);
    this.scrollSelectedRowIntoView();
  }

  private scrollSelectedRowIntoView(): void {
    const { tbody } = this.getElements();
    if (!tbody) return;
    const selected = tbody.querySelector("tr[data-selected='true']") as HTMLTableRowElement | null;
    selected?.scrollIntoView({ block: 'nearest' });
  }

  private applyFilterFromInput(): void {
    const q = (this.input?.value ?? '').trim().toLowerCase();
    if (!q) {
      this.visibleRows = [...this.allRows];
      this.selectedIndex = 0;
      this.renderVisibleRows();
      return;
    }
    this.visibleRows = this.allRows.filter((row) => {
      const hay = `${row.key} ${row.value} ${row.status}`.toLowerCase();
      return hay.includes(q);
    });
    this.selectedIndex = 0;
    this.renderVisibleRows();
  }

  private moveSelection(delta: number): void {
    if (this.visibleRows.length === 0) return;
    const next = this.selectedIndex + delta;
    this.selectedIndex = Math.max(0, Math.min(this.visibleRows.length - 1, next));
    this.renderVisibleRows();
  }

  private setSelection(index: number): void {
    if (this.visibleRows.length === 0) return;
    this.selectedIndex = Math.max(0, Math.min(this.visibleRows.length - 1, index));
    this.renderVisibleRows();
  }

  private gotoSection(hash: '#dag-3d-stage' | '#dag-log-xterm'): void {
    if (window.location.hash !== hash) window.location.hash = hash;
  }

  private cycleMode(): void {
    this.mode = this.mode === 'browse' ? 'filter' : 'browse';
    this.syncForm();
  }

  private actionSet(): Array<{ label: string; aria: string; run: () => void | Promise<void> }> {
    if (this.mode === 'filter') {
      return [
        { label: 'Refresh', aria: 'Refresh', run: () => void this.reload() },
        { label: 'Apply', aria: 'Apply', run: () => this.applyFilterFromInput() },
        {
          label: 'Clear',
          aria: 'Clear',
          run: () => {
            if (this.input) this.input.value = '';
            this.applyFilterFromInput();
          },
        },
        { label: 'Top', aria: 'Top', run: () => this.setSelection(0) },
        { label: 'Bottom', aria: 'Bottom', run: () => this.setSelection(this.visibleRows.length - 1) },
        { label: 'Stage', aria: 'Stage', run: () => this.gotoSection('#dag-3d-stage') },
        { label: 'Log', aria: 'Log', run: () => this.gotoSection('#dag-log-xterm') },
        { label: 'Down', aria: 'Down', run: () => this.moveSelection(1) },
      ];
    }
    return [
      { label: 'Refresh', aria: 'Refresh', run: () => void this.reload() },
      { label: 'Up', aria: 'Up', run: () => this.moveSelection(-1) },
      { label: 'Down', aria: 'Down', run: () => this.moveSelection(1) },
      { label: 'Top', aria: 'Top', run: () => this.setSelection(0) },
      { label: 'Bottom', aria: 'Bottom', run: () => this.setSelection(this.visibleRows.length - 1) },
      { label: 'Stage', aria: 'Stage', run: () => this.gotoSection('#dag-3d-stage') },
      { label: 'Log', aria: 'Log', run: () => this.gotoSection('#dag-log-xterm') },
      {
        label: 'Clear',
        aria: 'Clear',
        run: () => {
          if (this.input) this.input.value = '';
          this.applyFilterFromInput();
        },
      },
    ];
  }

  private syncForm(): void {
    const actions = this.actionSet();
    for (let i = 0; i < this.actionButtons.length; i += 1) {
      const action = actions[i];
      const button = this.actionButtons[i];
      if (!action || !button) continue;
      button.textContent = `${i + 1}:${action.label}`;
      button.setAttribute('aria-label', action.aria);
      button.onclick = () => {
        void action.run();
      };
    }
    if (this.modeButton) {
      this.modeButton.textContent = `9:Mode: ${this.mode === 'browse' ? 'Browse' : 'Filter'}`;
      this.modeButton.setAttribute('data-mode', this.mode);
    }
    if (this.input) {
      this.input.placeholder = this.mode === 'browse' ? 'Filter table rows' : 'Filter key/value/status';
    }
    if (this.submitButton) {
      this.submitButton.textContent = '10:Apply';
    }
  }

  private bindModeForm(): void {
    this.form = this.container.querySelector("form[data-mode-form='table']") as HTMLFormElement | null;
    if (!this.form) return;
    this.modeButton = this.form.querySelector("button[aria-label='Table Mode']") as HTMLButtonElement | null;
    this.input = this.form.querySelector("input[aria-label='Table Query Input']") as HTMLInputElement | null;
    this.submitButton = this.form.querySelector("button[aria-label='Table Submit']") as HTMLButtonElement | null;
    this.actionButtons = [
      this.form.querySelector("button[aria-label='Table Thumb 1']"),
      this.form.querySelector("button[aria-label='Table Thumb 2']"),
      this.form.querySelector("button[aria-label='Table Thumb 3']"),
      this.form.querySelector("button[aria-label='Table Thumb 4']"),
      this.form.querySelector("button[aria-label='Table Thumb 5']"),
      this.form.querySelector("button[aria-label='Table Thumb 6']"),
      this.form.querySelector("button[aria-label='Table Thumb 7']"),
      this.form.querySelector("button[aria-label='Table Thumb 8']"),
    ].filter((el): el is HTMLButtonElement => !!el);

    this.modeButton?.addEventListener('click', () => this.cycleMode());
    this.submitButton?.addEventListener('click', () => this.applyFilterFromInput());
    this.form.addEventListener('submit', (event) => {
      event.preventDefault();
      this.applyFilterFromInput();
    });
    this.input?.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') return;
      event.preventDefault();
      this.applyFilterFromInput();
    });

    this.syncForm();
  }

  setVisible(visible: boolean): void {
    const table = this.container.querySelector("table[aria-label='DAG Table']") as HTMLTableElement | null;
    if (!table) return;
    if (visible) {
      this.syncForm();
      if (Date.now() < this.retryBlockedUntil) {
        this.renderVisibleRows();
        table.setAttribute('data-ready', this.lastLoadFailed ? 'error' : 'true');
        return;
      }
      if (this.allRows.length === 0 || this.lastLoadFailed) {
        void this.ensureRowsLoaded(table);
      } else {
        this.renderVisibleRows();
        table.setAttribute('data-ready', 'true');
      }
    }
  }

  private ensureRowsLoaded(table: HTMLTableElement): Promise<void> {
    if (this.loadPromise) return this.loadPromise;
    this.loadPromise = this.loadRows(table).finally(() => {
      this.loadPromise = null;
    });
    return this.loadPromise;
  }

  private isStartupApiReadyHint(): boolean {
    try {
      return window.sessionStorage.getItem(this.apiReadyStorageKey) === '1';
    } catch {
      return false;
    }
  }

  private getFallbackRows(): TableRow[] {
    return [
      { key: 'query_error', value: 'api_unavailable', status: 'WARN' },
      { key: 'ui_status', value: 'stage_and_menu_available', status: 'OK' },
      { key: 'dev_hint', value: 'start ./dialtone.sh dag serve src_v3 for live api', status: 'INFO' },
      { key: 'node_count', value: '7', status: 'MOCK' },
      { key: 'edge_count', value: '7', status: 'MOCK' },
      { key: 'layer_count', value: '2', status: 'MOCK' },
      { key: 'graph_edge_match_count', value: '7', status: 'MOCK' },
      { key: 'rank_violation_count', value: '0', status: 'MOCK' },
    ];
  }

  private async fetchRowsWithRetry(maxAttempts = 4): Promise<TableRow[]> {
    const startupApiReady = this.isStartupApiReadyHint();
    const attempts = startupApiReady ? maxAttempts : 1;
    let lastErr: Error | null = null;
    for (let attempt = 1; attempt <= attempts; attempt += 1) {
      let timeout = 0;
      try {
        const controller = new AbortController();
        timeout = window.setTimeout(() => controller.abort(), 1200);
        const res = await fetch('/api/dag-table', {
          headers: {
            Accept: 'application/json',
          },
          signal: controller.signal,
        });
        if (!res.ok) {
          throw new Error(`failed to load dag table: ${res.status}`);
        }
        const contentType = res.headers.get('content-type') ?? '';
        if (!contentType.includes('application/json')) {
          throw new Error(`invalid dag table content-type: ${contentType || 'unknown'}`);
        }
        const body = (await res.json()) as { rows?: TableRow[] };
        return Array.isArray(body.rows) ? body.rows : [];
      } catch (err) {
        lastErr = err instanceof Error ? err : new Error(String(err));
        if (attempt < attempts) {
          await new Promise((resolve) => window.setTimeout(resolve, attempt * 250));
        }
      } finally {
        if (timeout) window.clearTimeout(timeout);
      }
    }
    throw lastErr ?? new Error('failed to load dag table');
  }

  private async reload(): Promise<void> {
    const { table } = this.getElements();
    if (!table) return;
    await this.loadRows(table);
  }

  private async loadRows(table: HTMLTableElement): Promise<void> {
    table.setAttribute('data-ready', 'loading');
    try {
      this.allRows = await this.fetchRowsWithRetry();
      this.lastLoadFailed = false;
      this.visibleRows = [...this.allRows];
      this.selectedIndex = 0;
      this.renderVisibleRows();
      table.setAttribute('data-ready', 'true');
    } catch (err) {
      console.error('[DAG Table] query failed', err);
      this.lastLoadFailed = true;
      this.retryBlockedUntil = Date.now() + 5000;
      this.allRows = this.getFallbackRows();
      this.visibleRows = [...this.allRows];
      this.selectedIndex = 0;
      this.renderVisibleRows();
      table.setAttribute('data-ready', 'error');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
