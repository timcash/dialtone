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
    const table = this.container.querySelector("table") as HTMLTableElement | null;
    if (!table) return { table: null, tbody: null as HTMLTableSectionElement | null };
    const tbody = table.querySelector('tbody');
    return { table, tbody };
  }

  private renderVisibleRows(): void {
    const { table } = this.getElements();
    if (!table) return;
    console.log('[TableControl] renderVisibleRows, total rows:', this.allRows.length);
    table.setAttribute('data-ready', 'true');
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.renderVisibleRows();
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
