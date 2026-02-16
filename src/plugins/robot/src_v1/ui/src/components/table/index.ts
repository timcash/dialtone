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
    const rows: TableRow[] = [];
    
    // Core System Stats
    if (data.uptime) rows.push({ key: 'uptime', value: data.uptime, status: 'LIVE' });
    if (data.nats_total !== undefined) rows.push({ key: 'nats_messages', value: String(data.nats_total), status: 'LIVE' });
    if (data.connections !== undefined) rows.push({ key: 'nats_clients', value: String(data.connections), status: 'LIVE' });
    
    // Robot Specific Telemetry
    if (data.lat !== undefined) rows.push({ key: 'gps_lat', value: data.lat.toFixed(6), status: 'LIVE' });
    if (data.lon !== undefined) rows.push({ key: 'gps_lon', value: data.lon.toFixed(6), status: 'LIVE' });
    if (data.alt !== undefined) rows.push({ key: 'gps_alt', value: data.alt.toFixed(1) + 'm', status: 'LIVE' });
    if (data.sats !== undefined) rows.push({ key: 'gps_sats', value: String(data.sats), status: 'LIVE' });
    if (data.battery !== undefined) rows.push({ key: 'battery', value: data.battery.toFixed(1) + 'V', status: 'LIVE' });
    
    // Add any other dynamic fields
    Object.entries(data).forEach(([key, value]) => {
      const skip = ['uptime', 'nats_total', 'connections', 'lat', 'lon', 'alt', 'sats', 'battery', 'roll', 'pitch', 'yaw', 'os', 'arch', 'caller', 'in_msgs', 'out_msgs', 'in_bytes', 'out_bytes'];
      if (!skip.includes(key)) {
        rows.push({ key, value: String(value), status: 'LIVE' });
      }
    });

    this.allRows = rows;
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
    console.log('[Table] renderRows called, visible:', this.visible, 'rows:', this.allRows.length);
    const table = this.container.querySelector("table[aria-label='Robot Table']") as HTMLTableElement | null;
    if (!table) {
      console.error('[Table] Table element not found in container');
      return;
    }
    const tbody = table.querySelector('tbody');
    if (!tbody) {
      console.error('[Table] tbody not found in table');
      return;
    }

    tbody.innerHTML = this.allRows
      .map((r) => `<tr><td>${r.key}</td><td>${r.value}</td><td>${r.status}</td></tr>`)
      .join('');
    
    console.log('[Table] setting data-ready=true');
    table.setAttribute('data-ready', 'true');
  }

  setVisible(visible: boolean): void {
    console.log('[Table] setVisible:', visible);
    this.visible = visible;
    if (visible) {
      this.renderRows();
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
