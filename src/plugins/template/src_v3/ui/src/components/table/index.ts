import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

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

  private form: HTMLFormElement | null = null;
  private modeButton: HTMLButtonElement | null = null;
  private actionButtons: HTMLButtonElement[] = [];
  private input: HTMLInputElement | null = null;
  private submitButton: HTMLButtonElement | null = null;

  constructor(private container: HTMLElement) {
    this.allRows = buildRows(120);
    this.visibleRows = [...this.allRows];
    this.bindModeForm();
  }

  dispose(): void {
    // no-op
  }

  private getElements() {
    const table = this.container.querySelector("table[aria-label='Template Table']") as HTMLTableElement | null;
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

  private gotoSection(hash: '#template-three-stage' | '#template-log-xterm'): void {
    if (window.location.hash !== hash) window.location.hash = hash;
  }

  private cycleMode(): void {
    this.mode = this.mode === 'browse' ? 'filter' : 'browse';
    this.syncForm();
  }

  private actionSet(): Array<{ label: string; aria: string; run: () => void }> {
    if (this.mode === 'filter') {
      return [
        { label: 'Refresh', aria: 'Refresh', run: () => this.refresh() },
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
        { label: 'Stage', aria: 'Stage', run: () => this.gotoSection('#template-three-stage') },
        { label: 'Log', aria: 'Log', run: () => this.gotoSection('#template-log-xterm') },
        { label: 'Down', aria: 'Down', run: () => this.moveSelection(1) },
      ];
    }
    return [
      { label: 'Refresh', aria: 'Refresh', run: () => this.refresh() },
      { label: 'Up', aria: 'Up', run: () => this.moveSelection(-1) },
      { label: 'Down', aria: 'Down', run: () => this.moveSelection(1) },
      { label: 'Top', aria: 'Top', run: () => this.setSelection(0) },
      { label: 'Bottom', aria: 'Bottom', run: () => this.setSelection(this.visibleRows.length - 1) },
      { label: 'Stage', aria: 'Stage', run: () => this.gotoSection('#template-three-stage') },
      { label: 'Log', aria: 'Log', run: () => this.gotoSection('#template-log-xterm') },
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
      button.onclick = () => action.run();
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

  private refresh(): void {
    this.allRows = buildRows(120);
    this.visibleRows = [...this.allRows];
    this.selectedIndex = 0;
    this.renderVisibleRows();
  }

  setVisible(visible: boolean): void {
    const table = this.container.querySelector("table[aria-label='Template Table']") as HTMLTableElement | null;
    if (!table) return;
    if (visible) {
      this.syncForm();
      this.renderVisibleRows();
      table.setAttribute('data-ready', 'true');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}

function buildRows(count: number): TableRow[] {
  const statuses = ['SYNCED', 'OPTIMAL', 'STABLE', 'ACTIVE', 'NORMAL', 'HEALTHY', 'WATCH'];
  const keys = [
    'system_clock',
    'network_latency',
    'cache_hit_rate',
    'worker_pool',
    'queue_depth',
    'db_replica_lag',
    'ingest_rate',
    'egress_rate',
    'auth_success',
    'event_backlog',
  ];
  const rows: TableRow[] = [];
  for (let i = 0; i < count; i += 1) {
    const key = `${keys[i % keys.length]}_${String(i + 1).padStart(3, '0')}`;
    const value = i % 2 === 0 ? `${1707525600 + i}` : `${(8 + (i % 91) * 0.1).toFixed(1)}ms`;
    rows.push({
      key,
      value,
      status: statuses[i % statuses.length],
    });
  }
  return rows;
}
