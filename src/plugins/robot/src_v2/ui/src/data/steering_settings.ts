export type SteeringSettingKey =
  | 'forwardThrottlePwm'
  | 'reverseThrottlePwm'
  | 'turnThrottlePwm'
  | 'leftSteeringPwm'
  | 'rightSteeringPwm'
  | 'forwardDurationMs'
  | 'reverseDurationMs'
  | 'turnDurationMs';

export type SteeringSettings = Record<SteeringSettingKey, number>;

const STORAGE_KEY = 'robot.steering.settings.v1';

export const STEERING_DEFAULTS: SteeringSettings = {
  forwardThrottlePwm: 2000,
  reverseThrottlePwm: 1000,
  turnThrottlePwm: 1800,
  leftSteeringPwm: 1000,
  rightSteeringPwm: 2000,
  forwardDurationMs: 2000,
  reverseDurationMs: 2000,
  turnDurationMs: 1200,
};

export const STEERING_KEY_ORDER: SteeringSettingKey[] = [
  'forwardThrottlePwm',
  'reverseThrottlePwm',
  'turnThrottlePwm',
  'leftSteeringPwm',
  'rightSteeringPwm',
  'forwardDurationMs',
  'reverseDurationMs',
  'turnDurationMs',
];

export const STEERING_LABELS: Record<SteeringSettingKey, string> = {
  forwardThrottlePwm: 'Forward Throttle PWM',
  reverseThrottlePwm: 'Reverse Throttle PWM',
  turnThrottlePwm: 'Turn Throttle PWM',
  leftSteeringPwm: 'Left Steering PWM',
  rightSteeringPwm: 'Right Steering PWM',
  forwardDurationMs: 'Forward Duration (ms)',
  reverseDurationMs: 'Reverse Duration (ms)',
  turnDurationMs: 'Turn Duration (ms)',
};

const MIN_MAX: Record<SteeringSettingKey, [number, number]> = {
  forwardThrottlePwm: [1000, 2000],
  reverseThrottlePwm: [1000, 2000],
  turnThrottlePwm: [1000, 2000],
  leftSteeringPwm: [1000, 2000],
  rightSteeringPwm: [1000, 2000],
  forwardDurationMs: [200, 5000],
  reverseDurationMs: [200, 5000],
  turnDurationMs: [200, 5000],
};

function clamp(key: SteeringSettingKey, value: number): number {
  const [min, max] = MIN_MAX[key];
  return Math.max(min, Math.min(max, Math.round(value)));
}

export function loadSteeringSettings(): SteeringSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...STEERING_DEFAULTS };
    const parsed = JSON.parse(raw) as Partial<SteeringSettings>;
    const merged: SteeringSettings = { ...STEERING_DEFAULTS };
    for (const key of STEERING_KEY_ORDER) {
      const next = parsed[key];
      if (typeof next === 'number' && Number.isFinite(next)) merged[key] = clamp(key, next);
    }
    return merged;
  } catch {
    return { ...STEERING_DEFAULTS };
  }
}

export function saveSteeringSettings(settings: SteeringSettings): void {
  const normalized: SteeringSettings = { ...STEERING_DEFAULTS };
  for (const key of STEERING_KEY_ORDER) normalized[key] = clamp(key, settings[key]);
  localStorage.setItem(STORAGE_KEY, JSON.stringify(normalized));
}

export function resetSteeringSettings(): SteeringSettings {
  const defaults = { ...STEERING_DEFAULTS };
  saveSteeringSettings(defaults);
  return defaults;
}

export function withAdjustedSetting(
  settings: SteeringSettings,
  key: SteeringSettingKey,
  delta: number
): SteeringSettings {
  const next: SteeringSettings = { ...settings };
  next[key] = clamp(key, next[key] + delta);
  return next;
}
