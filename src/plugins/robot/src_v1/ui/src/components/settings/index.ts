import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { registerButtons, renderButtons } from '../../buttons';

export function mountSettings(container: HTMLElement): VisualizationControl {
  const content = container.querySelector('.settings-primary');
  if (content) {
    content.innerHTML = `
      <h2>Robot Settings</h2>
      <p>Configure interface preferences.</p>
      <div id="settings-list" style="display: flex; flex-direction: column; gap: 12px; margin-top: 20px;"></div>
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
    if (chatlog) chatlog.hidden = !get('robot.chatlog.enabled');
    
    // Re-render buttons to update labels/state
    renderButtons('settings');
  };

  // Register Buttons
  registerButtons('settings', ['Config'], {
    'Config': [
      {
        label: `Chatlog: ${get('robot.chatlog.enabled') ? 'ON' : 'OFF'}`,
        action: () => set('robot.chatlog.enabled', !get('robot.chatlog.enabled')),
        active: get('robot.chatlog.enabled')
      },
      null, null, null, null, null, null, null // Fill empty slots
    ]
  });

  // Also render list in primary area
  const list = document.getElementById('settings-list');
  if (list) {
    const btn = document.createElement('button');
    btn.className = 'menu-button';
    btn.textContent = `Toggle Chatlog`;
    btn.onclick = () => {
        set('robot.chatlog.enabled', !get('robot.chatlog.enabled'));
        btn.textContent = `Toggle Chatlog: ${get('robot.chatlog.enabled') ? 'ON' : 'OFF'}`;
    };
    list.appendChild(btn);
  }

  renderButtons('settings');
  applySettings(); // Initial apply

  return {
    dispose: () => {},
    setVisible: (v) => {
        if (v) renderButtons('settings');
    }
  };
}
