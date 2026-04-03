import { VisualizationControl, VisibilityMixin } from "@ui/ui";
import { registerButtons, renderButtons } from "../../util/buttons";
import { logInfo } from "../../util/logging";

class TableControl implements VisualizationControl {
    private ws: WebSocket | null = null;
    private stoppingNames = new Set<string>();
    private startingNames = new Set<string>();
    private isVisible = false;

    constructor(private container: HTMLElement) {
        this.setupButtons();
        
        const tbody = this.container.querySelector('#node-rows') as HTMLElement;
        if (tbody) {
            tbody.onclick = (e) => {
                const target = e.target as HTMLElement;
                const stopBtn = target.closest('.stop-btn') as HTMLButtonElement;
                if (stopBtn) {
                    const name = stopBtn.dataset.name;
                    if (name) {
                        logInfo('ui/table', `[UI_ACTION] User clicked STOP for: ${name}`);
                        this.stopNode(name);
                    }
                }
                const startBtn = target.closest('.start-btn') as HTMLButtonElement;
                if (startBtn) {
                    const name = startBtn.dataset.name;
                    if (name) {
                        logInfo('ui/table', `[UI_ACTION] User clicked START for: ${name}`);
                        this.startNode(name);
                    }
                }
                const terminalBtn = target.closest('.terminal-btn') as HTMLButtonElement;
                if (terminalBtn) {
                    const name = terminalBtn.dataset.name;
                    if (name) {
                        logInfo('ui/table', `[UI_ACTION] User clicked TERMINAL for: ${name}`);
                        this.openTerminal(name);
                    }
                }
                const deleteBtn = target.closest('.delete-btn') as HTMLButtonElement;
                if (deleteBtn) {
                    const name = deleteBtn.dataset.name;
                    if (name) {
                        logInfo('ui/table', `[UI_ACTION] User clicked DELETE for: ${name}`);
                        this.deleteNode(name);
                    }
                }
            };
        }
    }

    private setupButtons() {
        registerButtons('table', ['Browse'], {
            'Browse': [
                { label: 'Refresh', action: () => this.refresh() },
                { label: 'Spawn', action: () => this.spawnNode() },
                null, null, null, null, null, null
            ]
        });
    }

    private async spawnNode() {
        const name = prompt("Enter WSL Instance Name:", "wsl-node-" + Math.floor(Math.random() * 1000));
        if (!name) return;
        logInfo('ui/table', `[UI_ACTION] User requested SPAWN for: ${name}`);
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
    }

    private connectWS() {
        if (this.ws) return;
        logInfo('ui/table', '[WSL] Connecting to WebSocket...');
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
        this.ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'list' && Array.isArray(msg.data)) {
                    this.renderRows(msg.data);
                }
            } catch (e) {
                console.error('[WSL] WS parse error', e);
            }
        };
        this.ws.onclose = () => {
            this.ws = null;
            if (this.isVisible) {
                setTimeout(() => this.connectWS(), 2000);
            }
        };
    }

    private async refresh() {
        try {
            const res = await fetch('/api/instances');
            const data = await res.json();
            if (Array.isArray(data)) {
                this.renderRows(data);
            }
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
                // Keep it in stopping state
            } else if (inst.state === 'Stopped') {
                this.stoppingNames.delete(inst.name);
            }
            if (inst.state === 'Running') {
                this.startingNames.delete(inst.name);
            }
        });

        tbody.innerHTML = instances.map(inst => {
            const isStopping = this.stoppingNames.has(inst.name);
            const isStarting = this.startingNames.has(inst.name);
            const isRunning = inst.state === 'Running';
            const statusColor = inst.state === 'Running' ? '#00ff88' : (inst.state === 'Stopped' ? '#ff4444' : '#ff8800');
            const stopBtnClass = isStopping ? 'stop-btn is-stopping' : 'stop-btn';
            const stopBtnText = isStopping ? 'STOPPING...' : 'STOP';
            const startBtnClass = isStarting ? 'start-btn is-starting' : 'start-btn';
            const startBtnText = isStarting ? 'STARTING...' : 'START';

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
                        <button class="terminal-btn" data-name="${inst.name}" aria-label="Open Terminal ${inst.name}" style="background: #0d2c1f; color: #8cffc5; border: 1px solid #1f6b4a; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px; margin-right: 4px;">TERMINAL</button>
                        ${isRunning
                            ? `<button class="${stopBtnClass}" data-name="${inst.name}" aria-label="Stop Node ${inst.name}" style="background: #331111; color: #ff4444; border: 1px solid #552222; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px; margin-right: 4px;">${stopBtnText}</button>`
                            : `<button class="${startBtnClass}" data-name="${inst.name}" aria-label="Start Node ${inst.name}" style="background: #102d20; color: #00ff88; border: 1px solid #1f6b4a; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px; margin-right: 4px;">${startBtnText}</button>`}
                        <button class="delete-btn" data-name="${inst.name}" aria-label="Delete Node ${inst.name}" style="background: #222; color: #888; border: 1px solid #444; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px;">DELETE</button>
                    </td>
                </tr>
            `;
        }).join('');
    }

    private async startNode(name: string) {
        this.startingNames.add(name);
        this.refresh();
        try {
            await fetch(`/api/start?name=${encodeURIComponent(name)}`);
        } catch (e) {
            console.error('[WSL] Start failed', e);
            this.startingNames.delete(name);
        }
    }

    private async stopNode(name: string) {
        this.stoppingNames.add(name);
        this.startingNames.delete(name);
        this.refresh();
        try {
            await fetch(`/api/stop?name=${encodeURIComponent(name)}`);
        } catch (e) {
            console.error('[WSL] Stop failed', e);
            this.stoppingNames.delete(name);
        }
    }

    private async openTerminal(name: string) {
        try {
            const res = await fetch(`/api/open-terminal?name=${encodeURIComponent(name)}`);
            if (!res.ok) {
                throw new Error(await res.text());
            }
        } catch (e) {
            console.error('[WSL] Open terminal failed', e);
        }
    }

    private async deleteNode(name: string) {
        if (!confirm(`Really delete ${name}?`)) return;
        try {
            await fetch(`/api/delete?name=${encodeURIComponent(name)}`);
            this.refresh();
        } catch (e) {
            console.error('[WSL] Delete failed', e);
        }
    }

    setVisible(visible: boolean) {
        this.isVisible = visible;
        if (visible) {
            this.connectWS();
            this.refresh();
            renderButtons('table');
        } else {
            if (this.ws) {
                this.ws.close();
                this.ws = null;
            }
        }
        VisibilityMixin.setVisible(this, visible, 'wsl-table');
    }

    dispose() {
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }
}

export function mountTable(container: HTMLElement): VisualizationControl {
    return new TableControl(container);
}
