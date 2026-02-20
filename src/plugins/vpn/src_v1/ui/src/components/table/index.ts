import { VisualizationControl } from '../../../../../../../plugins/ui/types';

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
    this.allRows = buildRows(100);
    window.addEventListener('resize', this.onResize);
  }

  dispose(): void {
    window.removeEventListener('resize', this.onResize);
  }

  private getElements() {
    const table = this.container.querySelector("table[aria-label='Template Table']") as HTMLTableElement | null;
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
    const table = this.container.querySelector("table[aria-label='Template Table']") as HTMLTableElement | null;
    if (!table) return;
    if (visible) {
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
