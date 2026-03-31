import { VisualizationControl } from '@ui/types';
import { registerButtons, renderButtons } from '../../buttons';
import { ROBOT_SECTION_IDS } from '../../section_ids';
import {
  loadSteeringSettings,
  resetSteeringSettings,
  saveSteeringSettings,
  STEERING_KEY_ORDER,
  STEERING_LABELS,
  type SteeringSettingKey,
  type SteeringSettings,
  withAdjustedSetting,
} from '../../data/steering_settings';

class SteeringSettingsControl implements VisualizationControl {
  private selectedIndex = 0;
  private settings: SteeringSettings = loadSteeringSettings();
  private status = 'Loaded';

  constructor(private container: HTMLElement) {
    registerButtons(ROBOT_SECTION_IDS.steeringSettings, ['Edit'], {
      Edit: [
        { label: 'Prev', action: () => this.selectRelative(-1) },
        { label: 'Next', action: () => this.selectRelative(1) },
        { label: '-100', action: () => this.adjustCurrent(-100) },
        { label: '-10', action: () => this.adjustCurrent(-10) },
        { label: '+10', action: () => this.adjustCurrent(10) },
        { label: '+100', action: () => this.adjustCurrent(100) },
        { label: 'Save', action: () => this.save() },
        { label: 'Reset', action: () => this.reset() },
      ],
    });
  }

  private currentKey(): SteeringSettingKey {
    return STEERING_KEY_ORDER[this.selectedIndex];
  }

  private selectRelative(delta: number): void {
    const total = STEERING_KEY_ORDER.length;
    this.selectedIndex = (this.selectedIndex + delta + total) % total;
    this.status = `Selected ${STEERING_LABELS[this.currentKey()]}`;
    this.render();
  }

  private adjustCurrent(delta: number): void {
    const key = this.currentKey();
    this.settings = withAdjustedSetting(this.settings, key, delta);
    this.status = `${STEERING_LABELS[key]} = ${this.settings[key]}`;
    this.render();
  }

  private save(): void {
    saveSteeringSettings(this.settings);
    this.status = 'Saved';
    this.render();
  }

  private reset(): void {
    this.settings = resetSteeringSettings();
    this.status = 'Reset to defaults';
    this.render();
  }

  private render(): void {
    const table = this.container.querySelector("table[aria-label='Steering Settings Table']") as HTMLTableElement | null;
    if (!table) return;
    const tbody = table.querySelector('tbody');
    if (!tbody) return;
    tbody.innerHTML = STEERING_KEY_ORDER.map((key, idx) => {
      const selected = idx === this.selectedIndex ? ' data-selected="true"' : '';
      return `<tr${selected}><td>${STEERING_LABELS[key]}</td><td>${this.settings[key]}</td><td>${idx === this.selectedIndex ? 'SELECTED' : ''}</td></tr>`;
    }).join('');
    table.setAttribute('data-selected-key', this.currentKey());
    table.setAttribute('data-selected-label', STEERING_LABELS[this.currentKey()]);
    const status = this.container.querySelector("[aria-label='Steering Settings Status']") as HTMLElement | null;
    if (status) {
      status.textContent = this.status;
      status.setAttribute('data-status', this.status);
    }
  }

  dispose(): void {}

  setVisible(visible: boolean): void {
    if (visible) {
      this.settings = loadSteeringSettings();
      this.render();
      renderButtons(ROBOT_SECTION_IDS.steeringSettings);
    }
  }
}

export function mountSteeringSettings(container: HTMLElement): VisualizationControl {
  return new SteeringSettingsControl(container);
}
