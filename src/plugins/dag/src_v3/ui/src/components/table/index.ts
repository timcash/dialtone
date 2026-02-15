import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

class TableControl implements VisualizationControl {
  private allRows: TableRow[] = [];
  private onResize = () => {
    if (!this.visible) return;
    this.renderVisibleRows();
  };
  private visible = false;

  constructor(private container: HTMLElement) {
    window.addEventListener('resize', this.onResize);
  }

  dispose(): void {
    window.removeEventListener('resize', this.onResize);
  }

  private getElements() {
    const table = this.container.querySelector("table[aria-label='DAG Table']") as HTMLTableElement | null;
    if (!table) return { table: null, tbody: null as HTMLTableSectionElement | null };
    const tbody = table.querySelector('tbody');
    return { table, tbody };
  }

  private renderRows(rows: TableRow[]): void {
    const { tbody } = this.getElements();
    if (!tbody) return;
    tbody.innerHTML = rows
      .map((r) => `<tr><td>${r.key}</td><td>${r.value}</td><td>${r.status}</td></tr>`)
      .join('');
  }

  private renderVisibleRows(): void {
    const { table, tbody } = this.getElements();
    if (!table || !tbody) return;
    if (this.allRows.length === 0) return;

    const tableFontSize = parseFloat(window.getComputedStyle(table).fontSize || '13');
    const rowHeight = Math.max(1, tableFontSize);
    const headHeight = Math.max(0, table.tHead?.getBoundingClientRect().height ?? 0);
    const available = Math.max(1, this.container.clientHeight - headHeight);
    const visibleCount = Math.max(1, Math.min(this.allRows.length, Math.floor(available / rowHeight)));

    table.setAttribute('data-total-rows', String(this.allRows.length));
    table.setAttribute('data-visible-rows', String(visibleCount));
    this.renderRows(this.allRows.slice(0, visibleCount));
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    const table = this.container.querySelector("table[aria-label='DAG Table']") as HTMLTableElement | null;
    if (!table) return;
    if (visible) {
      if (this.allRows.length === 0) {
        void this.loadRows(table);
      } else {
        this.renderVisibleRows();
        table.setAttribute('data-ready', 'true');
      }
    }
  }

  private async loadRows(table: HTMLTableElement): Promise<void> {
    table.setAttribute('data-ready', 'loading');
    try {
      const res = await fetch('/api/dag-table');
      if (!res.ok) {
        throw new Error(`failed to load dag table: ${res.status}`);
      }
      const body = (await res.json()) as { rows?: TableRow[] };
      const rows = Array.isArray(body.rows) ? body.rows : [];
      this.allRows = rows;
      this.renderVisibleRows();
      table.setAttribute('data-ready', 'true');
    } catch (err) {
      console.error('[DAG Table] query failed', err);
      this.allRows = [{ key: 'query_error', value: 'failed', status: 'ERROR' }];
      this.renderVisibleRows();
      table.setAttribute('data-ready', 'error');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
