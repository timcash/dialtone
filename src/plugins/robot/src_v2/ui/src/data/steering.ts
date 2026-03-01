import { sendCommand } from './connection';
import { loadSteeringSettings } from './steering_settings';

export type SteeringDirection = 'left' | 'right';

export function sendDriveUp(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_up', undefined, {
    throttlePwm: s.forwardThrottlePwm,
    steeringPwm: 1500,
    durationMs: s.forwardDurationMs,
  });
}

export function sendDriveDown(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_down', undefined, {
    throttlePwm: s.reverseThrottlePwm,
    steeringPwm: 1500,
    durationMs: s.reverseDurationMs,
  });
}

export function sendDriveLeft(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_left', undefined, {
    throttlePwm: 0,
    steeringPwm: s.leftSteeringPwm,
    durationMs: s.turnDurationMs,
    noStop: true,
    steeringOnly: true,
  });
}

export function sendDriveRight(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_right', undefined, {
    throttlePwm: 0,
    steeringPwm: s.rightSteeringPwm,
    durationMs: s.turnDurationMs,
    noStop: true,
    steeringOnly: true,
  });
}

export function sendDriveStop(): void {
  sendCommand('stop');
}

export class SteeringHoldController {
  private active: SteeringDirection | null = null;
  private timer: ReturnType<typeof setInterval> | null = null;
  private startedAt = 0;
  private lastMatchedAt = 0;
  private readonly confirmTimeoutMs = 1800;
  private readonly mismatchTimeoutMs = 1200;
  private readonly tolerancePwm = 140;

  constructor(private readonly onStateChange?: () => void) {}

  isActive(direction: SteeringDirection): boolean {
    return this.active === direction;
  }

  toggle(direction: SteeringDirection): void {
    if (this.active === direction) {
      this.stop(true);
      return;
    }
    this.start(direction);
  }

  stop(sendStop: boolean): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.active = null;
    this.startedAt = 0;
    this.lastMatchedAt = 0;
    if (sendStop) sendDriveStop();
    this.onStateChange?.();
  }

  private start(direction: SteeringDirection): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.active = direction;
    this.startedAt = Date.now();
    this.lastMatchedAt = 0;
    this.sendPulse(direction);
    this.timer = setInterval(() => {
      if (!this.active) return;
      this.sendPulse(this.active);
    }, this.intervalMs());
    this.onStateChange?.();
  }

  private sendPulse(direction: SteeringDirection): void {
    if (direction === 'left') {
      sendDriveLeft();
      return;
    }
    sendDriveRight();
  }

  private intervalMs(): number {
    const turn = loadSteeringSettings().turnDurationMs;
    const candidate = Math.floor(turn * 0.5);
    return Math.max(180, Math.min(700, candidate));
  }

  sendDriveUpWithCurrentSteering(): void {
    const s = loadSteeringSettings();
    sendCommand('drive_up', undefined, {
      throttlePwm: s.forwardThrottlePwm,
      steeringPwm: this.activeSteeringPWM(s),
      durationMs: s.forwardDurationMs,
      noStop: this.active !== null,
    });
  }

  sendDriveDownWithCurrentSteering(): void {
    const s = loadSteeringSettings();
    sendCommand('drive_down', undefined, {
      throttlePwm: s.reverseThrottlePwm,
      steeringPwm: this.activeSteeringPWM(s),
      durationMs: s.reverseDurationMs,
      noStop: this.active !== null,
    });
  }

  handleTelemetry(data: any): void {
    if (!this.active) return;
    if (!data || data.type !== 'CONTROL_FEEDBACK') return;
    const steering = Number(data.steering_raw);
    if (!Number.isFinite(steering) || steering <= 0) return;

    const now = Date.now();
    const s = loadSteeringSettings();
    const matchesLeft = steering <= s.leftSteeringPwm + this.tolerancePwm;
    const matchesRight = steering >= s.rightSteeringPwm - this.tolerancePwm;
    const matched = this.active === 'left' ? matchesLeft : matchesRight;

    if (matched) {
      this.lastMatchedAt = now;
      return;
    }

    if (this.lastMatchedAt === 0 && now-this.startedAt > this.confirmTimeoutMs) {
      this.stop(false);
      return;
    }
    if (this.lastMatchedAt > 0 && now-this.lastMatchedAt > this.mismatchTimeoutMs) {
      this.stop(false);
    }
  }

  private activeSteeringPWM(s: ReturnType<typeof loadSteeringSettings>): number {
    if (this.active === 'left') return s.leftSteeringPwm;
    if (this.active === 'right') return s.rightSteeringPwm;
    return 1500;
  }
}
