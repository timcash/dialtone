export class TableSection {
    private interval: number | null = null;
    private stoppingIds = new Set<string>();

    constructor(private container: HTMLElement) {}

    async mount() {
        // Use event delegation for the entire table body
        const tbody = this.container.querySelector('#node-rows') as HTMLElement;
        if (tbody) {
            tbody.onclick = (e) => {
                const target = e.target as HTMLElement;
                const stopBtn = target.closest('.stop-btn') as HTMLButtonElement;
                if (stopBtn) {
                    const id = stopBtn.dataset.id;
                    if (id) {
                        this.stopNode(id);
                    }
                }
            };
        }

        const startBtn = this.container.querySelector('#start-node') as HTMLButtonElement;
        if (startBtn) {
            startBtn.onclick = async () => {
                try {
                    const res = await fetch('/api/processes', { method: 'POST' });
                    await res.json();
                    this.updateSpreadsheet();
                } catch (e) {
                    console.error('[NIX] Spawn failed', e);
                }
            };
        }
        this.interval = window.setInterval(() => this.updateSpreadsheet(), 1000);
        this.updateSpreadsheet();
    }

    unmount() {
        if (this.interval) {
            clearInterval(this.interval);
            this.interval = null;
        }
    }

    setVisible(_visible: boolean) {}

    private async updateSpreadsheet() {
        try {
            const res = await fetch('/api/processes');
            const procs = await res.json();
            
            const tbody = this.container.querySelector('#node-rows') as HTMLElement;
            if (!tbody) return;

            if (!procs || !Array.isArray(procs) || procs.length === 0) {
                tbody.innerHTML = '<tr><td colspan="6" style="padding: 20px; text-align: center; opacity: 0.5;">No active nodes found</td></tr>';
                return;
            }

            // Clean up stoppingIds for nodes that are now stopped
            procs.forEach((p: any) => {
                if (p.status === 'stopped' && this.stoppingIds.has(p.id)) {
                    this.stoppingIds.delete(p.id);
                }
            });

            procs.sort((a: any, b: any) => a.id.localeCompare(b.id, undefined, { numeric: true, sensitivity: 'base' }));

            tbody.innerHTML = procs.map((p: any) => {
                const isStopping = this.stoppingIds.has(p.id);
                const lastLog = p.logs && p.logs.length > 0 ? p.logs[p.logs.length - 1] : 'Waiting for logs...';
                const statusColor = p.status === 'running' ? '#004422' : '#440000';
                const stopBtnClass = isStopping ? 'stop-btn is-stopping' : 'stop-btn';
                const stopBtnText = isStopping ? 'STOPPING...' : 'STOP';
                
                return `
                    <tr class="node-row" id="${p.id}" data-status="${p.status}" style="border-bottom: 1px solid #222;">
                        <td style="padding: 12px; font-weight: bold; color: #00ff88;">${p.id}</td>
                        <td style="padding: 12px; color: #aaa;">${p.pid || '-'}</td>
                        <td style="padding: 12px;">
                            <span class="status-badge" data-status-text="${p.status}" style="padding: 2px 6px; border-radius: 3px; background: ${statusColor}; font-size: 11px;">
                                ${p.status.toUpperCase()}
                            </span>
                        </td>
                        <td style="padding: 12px; color: #aaa;">${p.start_time || '-'}</td>
                        <td class="node-logs" style="padding: 12px; color: #888; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${lastLog}</td>
                        <td style="padding: 12px; text-align: right;">
                            <button class="${stopBtnClass}" data-id="${p.id}" aria-label="Stop Node ${p.id}" style="background: #331111; color: #ff4444; border: 1px solid #552222; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px;">${stopBtnText}</button>
                        </td>
                    </tr>`;
            }).join('');
        } catch (e) {
            console.error('[NIX] Update spreadsheet failed', e);
        }
    }

    private async stopNode(id: string) {
        this.stoppingIds.add(id);
        this.updateSpreadsheet(); // Update UI immediately
        try {
            await fetch(`/api/stop?id=${id}`);
            // We don't remove from stoppingIds here, we wait for the next updateSpreadsheet to confirm status is 'stopped'
        } catch (e) {
            console.error('[NIX] Failed to stop ' + id, e);
            this.stoppingIds.delete(id);
        }
        this.updateSpreadsheet();
    }
}