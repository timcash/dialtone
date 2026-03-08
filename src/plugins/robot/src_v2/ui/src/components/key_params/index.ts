import { VisualizationControl } from '@ui/types';

type ParamRow = {
  key: string;
  value: string;
  status: string;
};

const DEFAULT_ROWS: ParamRow[] = [
  { key: 'RCMAP_STEERING', value: '1', status: 'DEFAULT' },
  { key: 'RCMAP_THROTTLE', value: '3', status: 'DEFAULT' },
  { key: 'RCMAP_ROLL', value: '1', status: 'DEFAULT' },
  { key: 'RCMAP_PITCH', value: '2', status: 'DEFAULT' },
  { key: 'RCMAP_YAW', value: '4', status: 'DEFAULT' },
  { key: 'SERVO1_FUNCTION', value: '26', status: 'DEFAULT' },
  { key: 'SERVO3_FUNCTION', value: '70', status: 'DEFAULT' },
  { key: 'CRUISE_SPEED', value: '2', status: 'DEFAULT' },
  { key: 'CRUISE_THROTTLE', value: '30', status: 'DEFAULT' },
  { key: 'WP_SPEED', value: '2', status: 'DEFAULT' },
];

export function mountKeyParams(container: HTMLElement): VisualizationControl {
  const table = container.querySelector("table[aria-label='Key Params Table']") as HTMLTableElement | null;
  const tbody = container.querySelector('tbody');
  if (!tbody) {
    throw new Error('key params table body not found');
  }

  let disposed = false;
  let timer: number | null = null;

  const renderRows = (rows: ParamRow[]) => {
    tbody.innerHTML = '';
    if (table) table.setAttribute('data-row-count', String(rows.length));
    for (const row of rows) {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td>${row.key}</td><td>${row.value}</td><td>${row.status}</td>`;
      tbody.appendChild(tr);
    }
  };

  const refresh = async () => {
    if (disposed) return;
    try {
      const res = await fetch('/api/key-params', { cache: 'no-store' });
      if (!res.ok) {
        renderRows(DEFAULT_ROWS);
        return;
      }
      const data = await res.json();
      const params = (data && typeof data === 'object' ? (data.params as Record<string, any>) : {}) || {};
      const source = String((data && (data as any).source) || 'default').toLowerCase();
      const rowStatus = source === 'live' ? 'LIVE' : 'DEFAULT';
      const rows: ParamRow[] = DEFAULT_ROWS.map((r) => {
        const v = params[r.key];
        if (v === undefined || v === null || v === '') {
          return r;
        }
        return {
          key: r.key,
          value: String(v),
          status: rowStatus,
        };
      });
      renderRows(rows);
    } catch {
      renderRows(DEFAULT_ROWS);
    }
  };

  renderRows(DEFAULT_ROWS);
  void refresh();
  timer = window.setInterval(() => {
    void refresh();
  }, 10000);

  return {
    dispose() {
      disposed = true;
      if (timer !== null) {
        window.clearInterval(timer);
      }
    },
    setVisible(visible: boolean) {
      if (visible) {
        void refresh();
      }
    },
  };
}
