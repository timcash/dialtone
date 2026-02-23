import { VisualizationControl } from '@ui/types';

type MetadataRow = { key: string; value: string; state: string };
type RouteRow = { subdomain: string; target: string; tls: string; proxy: string };

class TableControl implements VisualizationControl {
  private metadataRows: MetadataRow[] = [];
  private routeRows: RouteRow[] = [];
  private onResize = () => {
    if (!this.visible) return;
    this.renderVisibleRows();
  };
  private visible = false;

  constructor(private container: HTMLElement) {
    this.metadataRows = buildMetadataRows();
    this.routeRows = buildRouteRows();
    window.addEventListener('resize', this.onResize);
  }

  dispose(): void {
    window.removeEventListener('resize', this.onResize);
  }

  private renderVisibleRows(): void {
    const tunnelTable = this.container.querySelector("[aria-label='Tunnel Table']") as HTMLTableElement | null;
    const routingTable = this.container.querySelector("[aria-label='Routing Table']") as HTMLTableElement | null;
    const tunnelBody = tunnelTable?.querySelector('tbody');
    const routingBody = routingTable?.querySelector('tbody');
    if (!tunnelTable || !routingTable || !tunnelBody || !routingBody) return;

    tunnelBody.innerHTML = this.metadataRows
      .map((row) => `<tr><td>${row.key}</td><td>${row.value}</td><td>${row.state}</td></tr>`)
      .join('');
    routingBody.innerHTML = this.routeRows
      .map((row) => `<tr><td>${row.subdomain}</td><td>${row.target}</td><td>${row.proxy}</td><td>${row.tls}</td></tr>`)
      .join('');

    tunnelTable.setAttribute('data-ready', 'true');
    console.log('[TableControl] status metadata rendered');
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

function buildMetadataRows(): MetadataRow[] {
  const now = new Date();
  return [
    { key: 'relay_hostname', value: window.location.hostname || 'localhost', state: 'ACTIVE' },
    { key: 'ui_build', value: 'src_v1', state: 'READY' },
    { key: 'timezone', value: Intl.DateTimeFormat().resolvedOptions().timeZone || 'unknown', state: 'SET' },
    { key: 'timestamp', value: now.toISOString(), state: 'SYNCED' },
    { key: 'user_agent', value: navigator.userAgent.slice(0, 48), state: 'OBSERVED' },
    { key: 'connection', value: navigator.onLine ? 'online' : 'offline', state: navigator.onLine ? 'HEALTHY' : 'DEGRADED' },
  ];
}

function buildRouteRows(): RouteRow[] {
  const host = window.location.hostname || 'relay.local';
  const base = host.split(':')[0];
  return [
    { subdomain: 'drone-1.dialtone.earth', target: 'http://drone-1:80', proxy: 'ENABLED', tls: 'FULL' },
    { subdomain: 'robot.dialtone.earth', target: `http://${base}:8080`, proxy: 'ENABLED', tls: 'FULL' },
    { subdomain: 'relay.dialtone.earth', target: `http://${base}:3000`, proxy: 'ENABLED', tls: 'FULL' },
  ];
}
