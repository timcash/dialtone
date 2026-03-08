import type { Menu } from './Menu';
import type { SectionManager } from './SectionManager';
import type { VisualizationControl, SectionOverlayConfig } from './types';

export type UISharedTemplateID = 'docs' | 'table' | 'three' | 'terminal' | 'camera';
export type UISharedTemplateMode = 'fullscreen' | 'calculator';
export type UISharedLegendKind = 'text' | 'telemetry';
export type UISharedUnderlayKind = UISharedTemplateID;

export type UISharedShellOptions = {
  underlay: UISharedUnderlayKind;
  mode?: UISharedTemplateMode;
  legend?: UISharedLegendKind | 'none';
  form?: boolean;
  chatlog?: boolean;
};

export type UISharedTemplate = {
  id: UISharedTemplateID;
  title: string;
  defaultMode: UISharedTemplateMode;
  legendKind: UISharedLegendKind;
  overlays: SectionOverlayConfig;
  render: () => string;
};

export type UISharedSectionEntry = {
  sectionID: string;
  template: UISharedTemplateID;
  title: string;
};

export type UISharedRegisterOptions = {
  sections: SectionManager;
  menu: Menu;
  entries: UISharedSectionEntry[];
  decorate?: (entry: UISharedSectionEntry, container: HTMLElement) => Promise<VisualizationControl | void> | VisualizationControl | void;
};

function textLegend(title: string, subtitle: string): string {
  return `
    <header class="overlay-legend shell-legend shell-legend-text" aria-label="Text Legend">
      <h1>${title}</h1>
      <p>${subtitle}</p>
    </header>
  `;
}

function telemetryLegend(rows: Array<[string, string, string?]>): string {
  const cells = rows
    .map(
      ([label, value, className]) => `
        <div>
          <dt>${label}</dt>
          <dd${className ? ` class="${className}"` : ''}>${value}</dd>
        </div>
      `,
    )
    .join('');
  return `
    <aside class="overlay-legend shell-legend shell-legend-telemetry" aria-label="Telemetry Legend">
      <dl>${cells}</dl>
    </aside>
  `;
}

function modeForm(prefix: string, labels: string[]): string {
  const buttons = labels
    .slice(0, 9)
    .map(
      (label, index) =>
        `<button type="button" aria-label="${prefix} ${label}">${index + 1}:${label}</button>`,
    )
    .join('');
  return `
    <form class="mode-form" data-mode-form-state="open" aria-label="${prefix} Mode Form">
      ${buttons}
      <input type="text" aria-label="${prefix} Input" placeholder="${prefix.toLowerCase()} command" />
      <button type="button" aria-label="${prefix} Submit">Submit</button>
    </form>
  `;
}

function renderUnderlay(kind: UISharedUnderlayKind): string {
  switch (kind) {
    case 'docs':
      return `
        <article class="docs-primary overlay-primary" aria-label="Docs Underlay">
          <h2>Docs Title</h2>
          <p>Starter shell for docs and onboarding content.</p>
          <p>Use this for narrative content, setup notes, or runbooks.</p>
        </article>
      `;
    case 'table':
      return `
        <div class="table-wrapper overlay-primary">
          <table class="telemetry-table" aria-label="Table Underlay">
            <thead>
              <tr><th>Run</th><th>Subject</th><th>Status</th><th>Duration</th></tr>
            </thead>
            <tbody>
              <tr><td>load</td><td>nats.browser</td><td>pass</td><td>48ms</td></tr>
              <tr><td>goto</td><td>managed-tab</td><td>pass</td><td>32ms</td></tr>
              <tr><td>type</td><td>fixture-input</td><td>pass</td><td>17ms</td></tr>
            </tbody>
          </table>
        </div>
      `;
    case 'three':
      return `<canvas class="three-stage overlay-primary" aria-label="Three Underlay"></canvas>`;
    case 'terminal':
      return `<div class="xterm-primary overlay-primary" aria-label="Terminal Underlay"></div>`;
    case 'camera':
      return `<img class="camera-stage overlay-primary" aria-label="Camera Underlay" alt="camera feed" />`;
  }
}

function renderDefaultForm(kind: UISharedUnderlayKind): string {
  switch (kind) {
    case 'table':
      return modeForm('Table', ['Refresh', 'Filter', 'Export', 'Sort', 'Focus', 'Details', 'Diff', 'Tail', 'Mode']);
    case 'three':
      return modeForm('Three', ['Back', 'Add', 'Link', 'Clear', 'Open', 'Rename', 'Focus', 'Labels', 'Mode']);
    case 'terminal':
      return modeForm('Terminal', ['Left', 'Right', 'Up', 'Down', 'Home', 'End', 'Select', 'Copy', 'Mode']);
    case 'camera':
      return modeForm('Camera', ['Feed A', 'Feed B', 'Wide', 'Zoom', 'IR', 'Map', 'Log', 'Mark', 'Mode']);
    case 'docs':
      return '';
  }
}

function renderDefaultLegend(kind: UISharedUnderlayKind, legend: UISharedLegendKind): string {
  if (legend === 'text') {
    switch (kind) {
      case 'docs':
        return textLegend('Documentation', 'Guides, notes, and runtime instructions');
      case 'three':
        return textLegend('Three Demo', 'Fullscreen scene without controls for overview or hero contexts');
      case 'table':
        return textLegend('Runs Table', 'Structured data surface with optional controls');
      case 'terminal':
        return textLegend('Signals', 'Terminal-style log surface with optional command form');
      case 'camera':
        return textLegend('Camera', 'Video/camera surface with overlay controls');
    }
  }
  switch (kind) {
    case 'docs':
      return textLegend('Documentation', 'Guides, notes, and runtime instructions');
    case 'table':
      return telemetryLegend([
        ['source', 'nats'],
        ['status', 'idle', 'table-status'],
        ['rows', '3'],
        ['rate', '5hz'],
        ['view', 'all'],
        ['errors', '0'],
        ['sort', 'none'],
        ['filter', 'none'],
      ]);
    case 'three':
      return telemetryLegend([
        ['scene', 'graph'],
        ['camera', 'orbit'],
        ['fps', '60'],
        ['nodes', '22'],
        ['edges', '43'],
        ['labels', 'on'],
        ['gpu', 'on'],
        ['mode', 'edit'],
      ]);
    case 'terminal':
      return telemetryLegend([
        ['stream', 'logs.test'],
        ['status', 'live', 'terminal-status'],
        ['level', 'info'],
        ['tail', 'on'],
        ['errors', '0'],
        ['warn', '0'],
        ['info', '12'],
        ['mode', 'cursor'],
      ]);
    case 'camera':
      return telemetryLegend([
        ['stream', 'primary'],
        ['codec', 'h264'],
        ['fps', '30'],
        ['latency', '46ms'],
        ['drops', '0'],
        ['bitrate', '4mbps'],
        ['res', '1280x720'],
        ['audio', 'off'],
      ]);
  }
}

export function getUISharedShellOverlays(options: UISharedShellOptions): SectionOverlayConfig {
  const kind = options.underlay;
  const overlays: SectionOverlayConfig = {
    primaryKind: kind,
    primary:
      kind === 'docs'
        ? '.docs-primary'
        : kind === 'table'
          ? '.table-wrapper'
          : kind === 'three'
            ? '.three-stage'
            : kind === 'terminal'
              ? '.xterm-primary'
              : '.camera-stage',
  };
  if (options.form) {
    overlays.form = '.mode-form';
  }
  if ((options.legend ?? 'telemetry') !== 'none') {
    overlays.legend =
      (options.legend ?? 'telemetry') === 'text' ? '.shell-legend-text' : '.shell-legend-telemetry';
  }
  if (options.chatlog) {
    overlays.chatlog = '.shell-chatlog';
  }
  return overlays;
}

export function renderUISharedShell(container: HTMLElement, options: UISharedShellOptions): void {
  const mode = options.mode ?? 'fullscreen';
  const legend = options.legend ?? 'telemetry';
  container.classList.remove('fullscreen', 'calculator');
  container.classList.add(mode);
  const parts = [renderUnderlay(options.underlay)];
  if (options.chatlog) {
    parts.push(`
      <aside class="shell-chatlog overlay-chatlog" aria-label="Chatlog Overlay" hidden>
        <div class="shell-chatlog-terminal" aria-label="Chatlog Terminal"></div>
      </aside>
    `);
  }
  if (options.form) {
    parts.push(renderDefaultForm(options.underlay));
  }
  if (legend !== 'none') {
    parts.push(renderDefaultLegend(options.underlay, legend));
  }
  container.innerHTML = parts.join('');
}

const templateMap: Record<UISharedTemplateID, UISharedTemplate> = {
  docs: {
    id: 'docs',
    title: 'Docs',
    defaultMode: 'fullscreen',
    legendKind: 'text',
    overlays: {
      ...getUISharedShellOverlays({ underlay: 'docs', mode: 'fullscreen', legend: 'text', form: false }),
    },
    render: () => renderShellMarkup({ underlay: 'docs', mode: 'fullscreen', legend: 'text', form: false }),
  },
  table: {
    id: 'table',
    title: 'Table',
    defaultMode: 'fullscreen',
    legendKind: 'telemetry',
    overlays: {
      ...getUISharedShellOverlays({ underlay: 'table', mode: 'fullscreen', legend: 'telemetry', form: true }),
    },
    render: () => renderShellMarkup({ underlay: 'table', mode: 'fullscreen', legend: 'telemetry', form: true }),
  },
  three: {
    id: 'three',
    title: 'Three',
    defaultMode: 'fullscreen',
    legendKind: 'telemetry',
    overlays: {
      ...getUISharedShellOverlays({ underlay: 'three', mode: 'fullscreen', legend: 'telemetry', form: true, chatlog: true }),
    },
    render: () => renderShellMarkup({ underlay: 'three', mode: 'fullscreen', legend: 'telemetry', form: true, chatlog: true }),
  },
  terminal: {
    id: 'terminal',
    title: 'Terminal',
    defaultMode: 'fullscreen',
    legendKind: 'telemetry',
    overlays: {
      ...getUISharedShellOverlays({ underlay: 'terminal', mode: 'fullscreen', legend: 'telemetry', form: true }),
    },
    render: () => renderShellMarkup({ underlay: 'terminal', mode: 'fullscreen', legend: 'telemetry', form: true }),
  },
  camera: {
    id: 'camera',
    title: 'Camera',
    defaultMode: 'fullscreen',
    legendKind: 'telemetry',
    overlays: {
      ...getUISharedShellOverlays({ underlay: 'camera', mode: 'fullscreen', legend: 'telemetry', form: true }),
    },
    render: () => renderShellMarkup({ underlay: 'camera', mode: 'fullscreen', legend: 'telemetry', form: true }),
  },
};

function renderShellMarkup(options: UISharedShellOptions): string {
  const temp = document.createElement('div');
  renderUISharedShell(temp, options);
  return temp.innerHTML;
}

export function getUISharedTemplate(id: UISharedTemplateID): UISharedTemplate {
  return templateMap[id];
}

export function getUISharedTemplateIDs(): UISharedTemplateID[] {
  return Object.keys(templateMap) as UISharedTemplateID[];
}

export function renderUISharedTemplate(container: HTMLElement, id: UISharedTemplateID): void {
  const tpl = getUISharedTemplate(id);
  container.classList.remove('fullscreen', 'calculator');
  container.classList.add(tpl.defaultMode);
  container.innerHTML = tpl.render();
}

function noopControl(): VisualizationControl {
  return {
    dispose: () => {},
    setVisible: () => {},
  };
}

export function registerUISharedSections(options: UISharedRegisterOptions): void {
  const { sections, menu, entries, decorate } = options;
  for (const entry of entries) {
    const template = getUISharedTemplate(entry.template);
    sections.register(entry.sectionID, {
      containerId: entry.sectionID,
      canonicalName: entry.sectionID,
      load: async () => {
        const container = document.getElementById(entry.sectionID);
        if (!container) {
          throw new Error(`${entry.sectionID} container not found`);
        }
        renderUISharedTemplate(container, entry.template);
        const ctl = decorate ? await decorate(entry, container) : undefined;
        return ctl ?? noopControl();
      },
      overlays: template.overlays,
    });

    menu.addButton(entry.title, `Open ${entry.title}`, () => {
      void sections.navigateTo(entry.sectionID);
    });
  }
}
