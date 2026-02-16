import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { NATS_CONNECTION, NATS_JSON_CODEC } from '../../main';

class ControlsControl implements VisualizationControl {
  private container: HTMLElement;

  constructor(container: HTMLElement) {
    this.container = container;
    this.attachEventListeners();
  }

  private attachEventListeners() {
    this.container.querySelector('#btn-arm')?.addEventListener('click', () => this.sendCommand('arm'));
    this.container.querySelector('#btn-disarm')?.addEventListener('click', () => this.sendCommand('disarm'));
    this.container.querySelector('#btn-manual')?.addEventListener('click', () => this.sendCommand('mode', 'manual'));
    this.container.querySelector('#btn-guided')?.addEventListener('click', () => this.sendCommand('mode', 'guided'));
  }

  private sendCommand(cmd: string, mode?: string) {
    if (!NATS_CONNECTION || !NATS_JSON_CODEC) {
      console.warn('NATS not connected. Command not sent.');
      return;
    }

    const payload: { cmd: string; mode?: string } = { cmd };
    if (mode) {
      payload.mode = mode;
    }

    NATS_CONNECTION.publish('rover.command', NATS_JSON_CODEC.encode(payload));
    console.log(`[Controls] Command sent: ${JSON.stringify(payload)}`);
  }

  dispose(): void {
    // No specific cleanup needed for event listeners attached this way
  }

  setVisible(_visible: boolean): void {
    // No specific visibility logic needed for this component
  }
}

export function mountControls(container: HTMLElement): VisualizationControl {
  return new ControlsControl(container);
}
