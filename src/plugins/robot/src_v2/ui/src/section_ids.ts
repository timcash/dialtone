export const ROBOT_SECTION_IDS = {
  hero: 'robot-hero-stage',
  docs: 'robot-docs-docs',
  table: 'robot-table-table',
  steeringSettings: 'robot-steering-settings-table',
  keyParams: 'robot-key-params-table',
  three: 'robot-three-stage',
  xterm: 'robot-xterm-xterm',
  video: 'robot-video-video',
  settings: 'robot-settings-button-list',
} as const;

export type RobotSectionKey = keyof typeof ROBOT_SECTION_IDS;
export type RobotSectionID = (typeof ROBOT_SECTION_IDS)[RobotSectionKey];

export const ROBOT_SECTION_ORDER: readonly RobotSectionID[] = [
  ROBOT_SECTION_IDS.hero,
  ROBOT_SECTION_IDS.docs,
  ROBOT_SECTION_IDS.table,
  ROBOT_SECTION_IDS.steeringSettings,
  ROBOT_SECTION_IDS.keyParams,
  ROBOT_SECTION_IDS.three,
  ROBOT_SECTION_IDS.xterm,
  ROBOT_SECTION_IDS.video,
  ROBOT_SECTION_IDS.settings,
] as const;

export const ROBOT_SECTION_HASH_ALIASES: Record<string, RobotSectionID> = {
  hero: ROBOT_SECTION_IDS.hero,
  docs: ROBOT_SECTION_IDS.docs,
  table: ROBOT_SECTION_IDS.table,
  telemetry: ROBOT_SECTION_IDS.table,
  'steering-settings': ROBOT_SECTION_IDS.steeringSettings,
  steering: ROBOT_SECTION_IDS.steeringSettings,
  'key-params': ROBOT_SECTION_IDS.keyParams,
  keyparams: ROBOT_SECTION_IDS.keyParams,
  params: ROBOT_SECTION_IDS.keyParams,
  three: ROBOT_SECTION_IDS.three,
  xterm: ROBOT_SECTION_IDS.xterm,
  terminal: ROBOT_SECTION_IDS.xterm,
  video: ROBOT_SECTION_IDS.video,
  camera: ROBOT_SECTION_IDS.video,
  settings: ROBOT_SECTION_IDS.settings,
  [ROBOT_SECTION_IDS.hero]: ROBOT_SECTION_IDS.hero,
  [ROBOT_SECTION_IDS.docs]: ROBOT_SECTION_IDS.docs,
  [ROBOT_SECTION_IDS.table]: ROBOT_SECTION_IDS.table,
  [ROBOT_SECTION_IDS.steeringSettings]: ROBOT_SECTION_IDS.steeringSettings,
  [ROBOT_SECTION_IDS.keyParams]: ROBOT_SECTION_IDS.keyParams,
  [ROBOT_SECTION_IDS.three]: ROBOT_SECTION_IDS.three,
  [ROBOT_SECTION_IDS.xterm]: ROBOT_SECTION_IDS.xterm,
  [ROBOT_SECTION_IDS.video]: ROBOT_SECTION_IDS.video,
  [ROBOT_SECTION_IDS.settings]: ROBOT_SECTION_IDS.settings,
};
