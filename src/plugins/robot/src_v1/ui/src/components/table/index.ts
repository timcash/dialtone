import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

class TableControl implements VisualizationControl {
  private allRows: TableRow[] = [];
  private visible = false;
  private ws: WebSocket | null = null;

  constructor(private container: HTMLElement) {
    this.connectWS();
  }

  private connectWS() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    this.ws = new WebSocket(`${protocol}//${host}/ws`);
    
    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.updateData(data);
      } catch (e) {
        console.error('Failed to parse WS message', e);
      }
    };

    this.ws.onclose = () => {
      if (this.visible) {
        setTimeout(() => this.connectWS(), 2000);
      }
    };
  }

  private updateData(data: any) {
    this.allRows = Object.entries(data).map(([key, value]) => ({
      key,
      value: String(value),
      status: 'LIVE'
    }));
    if (this.visible) {
      this.renderRows();
    }
  }

  dispose(): void {
    if (this.ws) {
      this.ws.close();
    }
  }

  private renderRows(): void {
    const table = this.container.querySelector("table[aria-label='Robot Table']") as HTMLTableElement | null;
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;

    tbody.innerHTML = this.allRows
      .map((r) => `<tr><td>${r.key}</td><td>${r.value}</td><td>${r.status}</td></tr>`)
      .join('');
    
    table.setAttribute('data-ready', 'true');
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.renderRows();
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
