import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

class TableControl implements VisualizationControl {
  private allRows: TableRow[] = [];
  private loadPromise: Promise<void> | null = null;
  private lastLoadFailed = false;
  private retryBlockedUntil = 0;
  private readonly apiReadyStorageKey = 'dag.src_v3.api_ready';

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
      if (Date.now() < this.retryBlockedUntil) {
        this.renderAllRows();
        table.setAttribute('data-ready', this.lastLoadFailed ? 'error' : 'true');
        return;
      }
      if (this.allRows.length === 0 || this.lastLoadFailed) {
        void this.ensureRowsLoaded(table);
      } else {
        this.renderAllRows();
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

  private async loadRows(table: HTMLTableElement): Promise<void> {
    table.setAttribute('data-ready', 'loading');
    try {
      this.allRows = await this.fetchRowsWithRetry();
      this.lastLoadFailed = false;
      this.renderAllRows();
      table.setAttribute('data-ready', 'true');
    } catch (err) {
      console.error('[DAG Table] query failed', err);
      this.lastLoadFailed = true;
      this.retryBlockedUntil = Date.now() + 5000;
      this.allRows = this.getFallbackRows();
      this.renderAllRows();
      table.setAttribute('data-ready', 'error');
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
