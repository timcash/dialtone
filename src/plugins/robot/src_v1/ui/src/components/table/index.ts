import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { addMavlinkListener } from '../../data/connection';

type TableRow = {
  key: string;
  value: string;
  status: string;
};

class TableControl implements VisualizationControl {
  private allRows = new Map<string, TableRow>();
  private visible = false;
  private unsubscribe: (() => void) | null = null;

  constructor(private container: HTMLElement) {
    this.subscribe();
  }

  private subscribe() {
    if (this.unsubscribe) return;
    this.unsubscribe = addMavlinkListener((data: any) => {
      this.updateData(data);
    });
  }

  private initDataListener() {
     // Deprecated, logic moved to subscribe
  }

  private updateData(data: any) {
    // Core System Stats (from server ticker, merged with Mavlink if present)
    if (data.uptime) this.allRows.set('uptime', { key: 'uptime', value: data.uptime, status: 'LIVE' });
    if (data.nats_total !== undefined) this.allRows.set('nats_messages', { key: 'nats_messages', value: String(data.nats_total), status: 'LIVE' });
    if (data.connections !== undefined) this.allRows.set('nats_clients', { key: 'nats_clients', value: String(data.connections), status: 'LIVE' });

    // Handle Mavlink messages
    if (data.type === 'HEARTBEAT') {
      this.allRows.set('mav_type', { key: 'mav_type', value: String(data.mav_type), status: 'MAV' });
      this.allRows.set('custom_mode', { key: 'custom_mode', value: String(data.custom_mode), status: 'MAV' });
      this.allRows.set('heartbeat_ts', { key: 'heartbeat_ts', value: String(data.timestamp), status: 'MAV' });
    } else if (data.lat !== undefined && data.lon !== undefined) {
      this.allRows.set('gps_lat', { key: 'gps_lat', value: data.lat.toFixed(6), status: 'MAV' });
      this.allRows.set('gps_lon', { key: 'gps_lon', value: data.lon.toFixed(6), status: 'MAV' });
      if (data.alt !== undefined) this.allRows.set('gps_alt', { key: 'gps_alt', value: data.alt.toFixed(1) + 'm', status: 'MAV' });
      if (data.relative_alt !== undefined) this.allRows.set('gps_relative_alt', { key: 'gps_relative_alt', value: data.relative_alt.toFixed(1) + 'm', status: 'MAV' });
      if (data.vx !== undefined) this.allRows.set('vx', { key: 'vx', value: data.vx.toFixed(2), status: 'MAV' });
      if (data.vy !== undefined) this.allRows.set('vy', { key: 'vy', value: data.vy.toFixed(2), status: 'MAV' });
      if (data.vz !== undefined) this.allRows.set('vz', { key: 'vz', value: data.vz.toFixed(2), status: 'MAV' });
      if (data.hdg !== undefined) this.allRows.set('hdg', { key: 'hdg', value: data.hdg.toFixed(1), status: 'MAV' });
    } else if (data.roll !== undefined && data.pitch !== undefined) {
      this.allRows.set('roll', { key: 'roll', value: data.roll.toFixed(3), status: 'MAV' });
      this.allRows.set('pitch', { key: 'pitch', value: data.pitch.toFixed(3), status: 'MAV' });
      this.allRows.set('yaw', { key: 'yaw', value: data.yaw.toFixed(3), status: 'MAV' });
    } else if (data.text !== undefined && data.severity !== undefined) {
      this.allRows.set('status_text', { key: 'status_text', value: data.text, status: `MAV_SEV_${data.severity}` });
    } else if (data.command !== undefined && data.result !== undefined) {
      this.allRows.set('command_ack_cmd', { key: 'command_ack_cmd', value: String(data.command), status: `MAV_ACK_${data.result}` });
    }
    
    if (this.visible) {
      this.renderRows();
    }
  }

  dispose(): void {
    if (this.unsubscribe) {
      this.unsubscribe();
    }
  }

  private renderRows(): void {
    const table = this.container.querySelector("table[aria-label='Robot Table']") as HTMLTableElement | null;
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;

    const rows = Array.from(this.allRows.values());
    tbody.innerHTML = rows
      .map((r) => `<tr><td>${r.key}</td><td>${r.value}</td><td>${r.status}</td></tr>`)
      .join('');
    
    table.setAttribute('data-ready', 'true');
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) {
      this.subscribe();
      this.renderRows();
    } else {
      if (this.unsubscribe) {
        this.unsubscribe();
        this.unsubscribe = null;
      }
    }
  }
}

export function mountTable(container: HTMLElement): VisualizationControl {
  return new TableControl(container);
}
