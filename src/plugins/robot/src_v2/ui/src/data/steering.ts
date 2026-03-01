import { sendCommand } from './connection';
import { loadSteeringSettings } from './steering_settings';

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
  sendCommand('drive_up', undefined, {
    throttlePwm: s.forwardThrottlePwm,
    steeringPwm: s.leftSteeringPwm,
    durationMs: s.forwardDurationMs,
  });
}

export function sendDriveRight(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_up', undefined, {
    throttlePwm: s.forwardThrottlePwm,
    steeringPwm: s.rightSteeringPwm,
    durationMs: s.forwardDurationMs,
  });
}

export function sendDriveDownLeft(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_down', undefined, {
    throttlePwm: s.reverseThrottlePwm,
    steeringPwm: s.leftSteeringPwm,
    durationMs: s.reverseDurationMs,
  });
}

export function sendDriveDownRight(): void {
  const s = loadSteeringSettings();
  sendCommand('drive_down', undefined, {
    throttlePwm: s.reverseThrottlePwm,
    steeringPwm: s.rightSteeringPwm,
    durationMs: s.reverseDurationMs,
  });
}
