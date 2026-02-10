export class TableSection {
    private ws: WebSocket | null = null;
    private stoppingNames = new Set<string>();

    constructor(private container: HTMLElement) {}

    async mount() {
        console.log('[WSL] TableSection mounting...');
        const tbody = this.container.querySelector('#node-rows') as HTMLElement;
        if (tbody) {
            tbody.onclick = (e) => {
                const target = e.target as HTMLElement;
                const stopBtn = target.closest('.stop-btn') as HTMLButtonElement;
                if (stopBtn) {
                    const name = stopBtn.dataset.name;
                    if (name) {
                        console.log(`[UI_ACTION] User clicked STOP for: ${name}`);
                        this.stopNode(name);
                    }
                }
                const deleteBtn = target.closest('.delete-btn') as HTMLButtonElement;
                if (deleteBtn) {
                    const name = deleteBtn.dataset.name;
                    if (name) {
                        console.log(`[UI_ACTION] User clicked DELETE for: ${name}`);
                        this.deleteNode(name);
                    }
                }
            };
        }

        const startBtn = this.container.querySelector('#start-node') as HTMLButtonElement;
        if (startBtn) {
            startBtn.onclick = async () => {
                const name = prompt("Enter WSL Instance Name:", "wsl-node-" + Math.floor(Math.random() * 1000));
                if (!name) return;
                console.log(`[UI_ACTION] User requested SPAWN for: ${name}`);
                try {
                    await fetch('/api/instances', { 
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ name })
                    });
                    this.refresh();
                } catch (e) {
                    console.error('[WSL] Spawn failed', e);
                }
            };
        }

        this.connectWS();
        this.refresh();
    }

    private connectWS() {
        console.log('[WSL] Connecting to WebSocket...');
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
        this.ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'list') {
                    this.renderRows(msg.data);
                }
            } catch (e) {
                console.error('[WSL] WS parse error', e);
            }
        };
        this.ws.onclose = () => setTimeout(() => this.connectWS(), 2000);
    }

    private async refresh() {
        try {
            const res = await fetch('/api/instances');
            const data = await res.json();
            this.renderRows(data);
        } catch (e) {
            console.error('[WSL] Refresh failed', e);
        }
    }

    private renderRows(instances: any[]) {
        const tbody = this.container.querySelector('#node-rows') as HTMLElement;
        if (!tbody) return;

        if (!instances || instances.length === 0) {
            tbody.innerHTML = '<tr><td colspan="6" style="padding: 20px; text-align: center; opacity: 0.5;">No active nodes found</td></tr>';
            return;
        }

        instances.forEach(inst => {
            if (inst.state !== 'Running' && inst.state !== 'Starting' && this.stoppingNames.has(inst.name)) {
                // Keep it in stopping state until it's actually stopped if we just clicked stop
            } else if (inst.state === 'Stopped') {
                this.stoppingNames.delete(inst.name);
            }
        });

        tbody.innerHTML = instances.map(inst => {
            const isStopping = this.stoppingNames.has(inst.name);
            const statusColor = inst.state === 'Running' ? '#00ff88' : (inst.state === 'Stopped' ? '#ff4444' : '#ff8800');
            const stopBtnClass = isStopping ? 'stop-btn is-stopping' : 'stop-btn';
            const stopBtnText = isStopping ? 'STOPPING...' : 'STOP';

            return `
                <tr style="border-bottom: 1px solid #222;">
                    <td style="padding: 12px; font-weight: bold; color: #00ff88;">${inst.name}</td>
                    <td style="padding: 12px;">
                        <span style="padding: 2px 6px; border-radius: 3px; background: ${statusColor}22; color: ${statusColor}; border: 1px solid ${statusColor}44; font-size: 11px;">
                            ${inst.state.toUpperCase()}
                        </span>
                    </td>
                    <td style="padding: 12px; color: #aaa;">${inst.version}</td>
                    <td style="padding: 12px; color: #aaa; font-family: monospace;">${inst.memory}</td>
                    <td style="padding: 12px; color: #aaa; font-family: monospace;">${inst.disk}</td>
                    <td style="padding: 12px; text-align: right;">
                        <button class="${stopBtnClass}" data-name="${inst.name}" aria-label="Stop Node ${inst.name}" style="background: #331111; color: #ff4444; border: 1px solid #552222; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px; margin-right: 4px;">${stopBtnText}</button>
                        <button class="delete-btn" data-name="${inst.name}" aria-label="Delete Node ${inst.name}" style="background: #222; color: #888; border: 1px solid #444; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px;">DELETE</button>
                    </td>
                </tr>
            `;
        }).join('');
    }

    private async stopNode(name: string) {
        this.stoppingNames.add(name);
        this.refresh();
        try {
            await fetch(`/api/stop?name=${name}`);
        } catch (e) {
            console.error('[WSL] Stop failed', e);
            this.stoppingNames.delete(name);
        }
    }

    private async deleteNode(name: string) {
        if (!confirm(`Really delete ${name}?`)) return;
        try {
            await fetch(`/api/delete?name=${name}`);
            this.refresh();
        } catch (e) {
            console.error('[WSL] Delete failed', e);
        }
    }
}
