export type ButtonAction = () => void | Promise<void>;

export interface ButtonDef {
  label: string;
  action: ButtonAction;
  active?: boolean; // For toggle state
}

export interface SectionButtons {
  [mode: string]: (ButtonDef | null)[]; // 8 buttons (1-8)
}

export interface SectionConfig {
  modes: string[]; // ['default', 'alt']
  buttons: SectionButtons;
  currentMode: string;
}

const configs: Record<string, SectionConfig> = {};

export function registerButtons(sectionId: string, modes: string[], buttons: SectionButtons) {
  configs[sectionId] = {
    modes,
    buttons,
    currentMode: modes[0],
  };
}

export function getButton(sectionId: string, index: number): ButtonDef | null {
  const cfg = configs[sectionId];
  if (!cfg) return null;
  const modeBtns = cfg.buttons[cfg.currentMode];
  if (!modeBtns || index >= modeBtns.length) return null;
  return modeBtns[index];
}

export function setMode(sectionId: string, mode: string) {
  const cfg = configs[sectionId];
  if (!cfg) return;
  if (cfg.modes.includes(mode)) {
    cfg.currentMode = mode;
    renderButtons(sectionId);
  }
}

export function toggleMode(sectionId: string) {
  const cfg = configs[sectionId];
  if (!cfg) return;
  const idx = cfg.modes.indexOf(cfg.currentMode);
  cfg.currentMode = cfg.modes[(idx + 1) % cfg.modes.length];
  renderButtons(sectionId);
}

export function renderButtons(sectionId: string) {
  const cfg = configs[sectionId];
  if (!cfg) return;

  const form = document.querySelector(`form[data-mode-form='${getFormId(sectionId)}']`);
  if (!form) return;

  const btns = Array.from(form.querySelectorAll('button'));
  // Indices 0-7 are action buttons (1-8)
  // Index 8 is Mode button (9)
  // Index 9 is Submit button (10) - usually handles input

  const modeBtns = cfg.buttons[cfg.currentMode] || [];

  for (let i = 0; i < 8; i++) {
    const btn = btns[i];
    if (!btn) continue;
    
    const def = modeBtns[i];
    if (def) {
      btn.textContent = def.label;
      btn.onclick = (e) => {
        e.preventDefault();
        def.action();
        // Re-render to update active state if needed
        if (def.active !== undefined) renderButtons(sectionId);
      };
      btn.disabled = false;
      btn.style.opacity = '1';
      if (def.active) {
        btn.setAttribute('data-active', 'true');
        btn.style.borderColor = 'var(--theme-primary)';
      } else {
        btn.removeAttribute('data-active');
        btn.style.borderColor = '';
      }
    } else {
      btn.textContent = '';
      btn.onclick = null;
      btn.disabled = true;
      btn.style.opacity = '0.3';
      btn.style.borderColor = '';
    }
  }

  // Update Mode Button (Index 8)
  const modeBtn = btns[8];
  if (modeBtn) {
    modeBtn.textContent = `Mode: ${cfg.currentMode}`;
    modeBtn.onclick = (e) => {
      e.preventDefault();
      toggleMode(sectionId);
    };
  }
}

// Helper to map section ID to form data-mode-form ID
// In index.html:
// hero -> hero
// table -> table
// three -> three
// xterm -> log  <-- Mismatch!
// video -> video
function getFormId(sectionId: string): string {
  if (sectionId === 'xterm') return 'log';
  return sectionId;
}

export function handleButtonKey(sectionId: string, keyIndex: number) {
  // keyIndex: 0-7 map to buttons 1-8
  // keyIndex: 8 maps to Mode
  if (keyIndex === 8) {
    toggleMode(sectionId);
    return;
  }
  const btn = getButton(sectionId, keyIndex);
  if (btn) btn.action();
}
