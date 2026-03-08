import { VisualizationControl } from '@ui/types';

export function mountSettings(container: HTMLElement): VisualizationControl {
  type UpdateStatus = {
    currentVersion?: string;
    latestVersion?: string;
    available?: boolean;
    checkedAt?: string;
  };

  const content = container.querySelector('.settings-primary');
  if (content) {
    content.innerHTML = `
      <h2>Robot Settings</h2>
      <p>Configure interface preferences.</p>
      <div class="settings-button-list" id="settings-list"></div>
    `;
  }

  // Define Settings Logic
  const get = (key: string) => localStorage.getItem(key) === 'true';
  const set = (key: string, val: boolean) => {
    localStorage.setItem(key, String(val));
    applySettings();
  };

  const applySettings = () => {
    // Apply chatlog
    const chatlog = document.querySelector('.three-chatlog') as HTMLElement;
    if (chatlog) {
      const enabled = get('robot.chatlog.enabled');
      chatlog.hidden = !enabled;
      chatlog.setAttribute('data-enabled', enabled ? 'true' : 'false');
    }
  };

  // Also render list in primary area
  const list = content?.querySelector('#settings-list') as HTMLElement | null;
  let updateBtn: HTMLButtonElement | null = null;
  if (list) {
    const chatlogBtn = document.createElement('button');
    chatlogBtn.className = 'menu-button';
    chatlogBtn.setAttribute('aria-label', 'Toggle Chatlog Button');
    chatlogBtn.textContent = 'Toggle Chatlog';
    chatlogBtn.onclick = () => {
        set('robot.chatlog.enabled', !get('robot.chatlog.enabled'));
        chatlogBtn.textContent = `Toggle Chatlog: ${get('robot.chatlog.enabled') ? 'ON' : 'OFF'}`;
    };
    list.appendChild(chatlogBtn);

    updateBtn = document.createElement('button');
    updateBtn.className = 'menu-button';
    updateBtn.type = 'button';
    updateBtn.setAttribute('aria-label', 'Robot Version Button');
    updateBtn.textContent = 'version:dev';
    list.appendChild(updateBtn);
  }

  const applyUpdateStatus = (status: UpdateStatus) => {
    const current = String(status.currentVersion ?? (window as any).__robotCurrentVersion ?? 'dev');
    const available = status.available === true;
    if (updateBtn) {
      updateBtn.textContent = available ? `version:${current}:update` : `version:${current}`;
      updateBtn.onclick = available
        ? () => {
            const reload = (window as any).robotReloadForUpdate;
            if (typeof reload === 'function') {
              reload();
            }
          }
        : null;
    }
  };

  const statusListener = (event: Event) => {
    const custom = event as CustomEvent<UpdateStatus>;
    applyUpdateStatus(custom.detail ?? {});
  };
  window.addEventListener('robot-update-status', statusListener as EventListener);
  applyUpdateStatus((window as any).__robotUpdateStatus ?? {});

  applySettings(); // Initial apply

  return {
    dispose: () => {
      window.removeEventListener('robot-update-status', statusListener as EventListener);
    },
    setVisible: (_v) => {}
  };
}
