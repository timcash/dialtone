import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

class TableControl implements VisualizationControl {
  private allRows: TableRow[] = [];

  constructor(private container: HTMLElement) {}

  dispose(): void {}

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

  private renderAllRows(): void {
    const { table, tbody } = this.getElements();
    if (!table || !tbody) return;
    if (this.allRows.length === 0) return;
    table.setAttribute('data-total-rows', String(this.allRows.length));
    table.setAttribute('data-visible-rows', String(this.allRows.length));
    this.renderRows(this.allRows);
  }

  setVisible(visible: boolean): void {
    const table = this.container.querySelector("table[aria-label='DAG Table']") as HTMLTableElement | null;
    if (!table) return;
    if (visible) {
      if (this.allRows.length === 0) {
        void this.loadRows(table);
      } else {
        this.renderAllRows();
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
      this.renderAllRows();
      table.setAttribute('data-ready', 'true');
    } catch (err) {
      console.error('[DAG Table] query failed', err);
      this.allRows = [
        { key: 'query_error', value: 'api_unavailable', status: 'WARN' },
        { key: 'dev_hint', value: 'start ./dialtone.sh dag serve src_v3 for live api', status: 'INFO' },
        { key: 'node_count', value: '7', status: 'MOCK' },
        { key: 'edge_count', value: '7', status: 'MOCK' },
        { key: 'layer_count', value: '2', status: 'MOCK' },
        { key: 'graph_edge_match_count', value: '7', status: 'MOCK' },
        { key: 'shortest_path_hops_root_to_leaf', value: '2', status: 'MOCK' },
        { key: 'rank_violation_count', value: '0', status: 'MOCK' },
      ];
      this.renderAllRows();
      table.setAttribute('data-ready', 'error');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
