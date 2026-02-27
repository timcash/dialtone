import type { SectionOverlayConfig } from './types';

export type UISharedTemplateID =
  | 'hero'
  | 'three-fullscreen'
  | 'three-calculator'
  | 'table'
  | 'camera'
  | 'docs'
  | 'terminal'
  | 'settings';

export type UISharedTemplate = {
  id: UISharedTemplateID;
  title: string;
  defaultMode: 'fullscreen' | 'calculator';
  overlays: SectionOverlayConfig;
  render: () => string;
};

function textHeader(title: string, subtitle: string): string {
  return `
    <header class="overlay text">
      <h1>${title}</h1>
      <h3>${subtitle}</h3>
    </header>
  `;
}

function legendHeader(pairs: Array<[string, string, string?]>): string {
  const cells = pairs
    .map(
      ([label, value, key]) => `
        <div>
          <dt>${label}</dt>
          <dd${key ? ` data-field="${key}"` : ''}>${value}</dd>
        </div>
      `
    )
    .join('');
  return `
    <header class="overlay legend">
      <dl>${cells}</dl>
    </header>
  `;
}

const templateMap: Record<UISharedTemplateID, UISharedTemplate> = {
  hero: {
    id: 'hero',
    title: 'Hero',
    defaultMode: 'fullscreen',
    overlays: {
      primaryKind: 'stage',
      primary: 'canvas',
      modeForm: 'form',
      legend: 'header.overlay'
    },
    render: () => `
      <canvas></canvas>
      ${legendHeader([
        ['mode', 'guided'],
        ['armed', 'false'],
        ['fps', '60'],
        ['latency', '22ms'],
        ['link', 'ok'],
        ['battery', '94%'],
        ['temp', '41c'],
        ['profile', 'dev']
      ])}
      <form>
        <button type="button">Arm</button>
        <button type="button">Disarm</button>
        <button type="button">Manual</button>
        <button type="button">Guided</button>
        <button type="button">Pause</button>
        <button type="button">Resume</button>
        <button type="button">Home</button>
        <button type="button">Land</button>
        <button type="button">Mode</button>
        <input type="text" placeholder="command" />
        <button type="button">Send</button>
      </form>
    `
  },
  'three-fullscreen': {
    id: 'three-fullscreen',
    title: 'Three Fullscreen',
    defaultMode: 'fullscreen',
    overlays: {
      primaryKind: 'stage',
      primary: 'canvas',
      legend: 'header.overlay'
    },
    render: () => `
      <canvas></canvas>
      ${legendHeader([
        ['scene', 'earth'],
        ['camera', 'orbit'],
        ['fps', '60'],
        ['nodes', '128'],
        ['lights', '3'],
        ['draws', '412'],
        ['gpu', 'on'],
        ['quality', 'high']
      ])}
    `
  },
  'three-calculator': {
    id: 'three-calculator',
    title: 'Three Calculator',
    defaultMode: 'calculator',
    overlays: {
      primaryKind: 'stage',
      primary: 'canvas',
      modeForm: 'form',
      legend: 'header.overlay'
    },
    render: () => `
      <canvas></canvas>
      ${legendHeader([
        ['scene', 'graph'],
        ['select', 'none'],
        ['fps', '58'],
        ['nodes', '22'],
        ['edges', '43'],
        ['labels', 'on'],
        ['grid', 'on'],
        ['mode', 'edit']
      ])}
      <form>
        <button type="button">Select</button>
        <button type="button" data-action="three-add">Add</button>
        <button type="button">Link</button>
        <button type="button">Delete</button>
        <button type="button">Group</button>
        <button type="button">Split</button>
        <button type="button">Frame</button>
        <button type="button">Labels</button>
        <button type="button">Mode</button>
        <input type="text" placeholder="label" />
        <button type="button">Apply</button>
      </form>
    `
  },
  table: {
    id: 'table',
    title: 'Table',
    defaultMode: 'calculator',
    overlays: {
      primaryKind: 'table',
      primary: 'table',
      modeForm: 'form',
      legend: 'header.overlay'
    },
    render: () => `
      <table>
        <thead><tr><th>Node</th><th>Status</th><th>Latency</th></tr></thead>
        <tbody>
          <tr><td>camera</td><td>running</td><td>17ms</td></tr>
          <tr><td>mavlink</td><td>running</td><td>12ms</td></tr>
          <tr><td>repl</td><td>running</td><td>9ms</td></tr>
        </tbody>
      </table>
      ${legendHeader([
        ['source', 'nats'],
        ['status', 'idle', 'table-status'],
        ['rows', '3'],
        ['rate', '5hz'],
        ['view', 'all'],
        ['errors', '0'],
        ['sort', 'none'],
        ['filter', 'none']
      ])}
      <form>
        <button type="button" data-action="table-refresh">Refresh</button>
        <button type="button">Filter</button>
        <button type="button">Export</button>
        <button type="button">Sort</button>
        <button type="button">Focus</button>
        <button type="button">Details</button>
        <button type="button">Diff</button>
        <button type="button">Tail</button>
        <button type="button">Mode</button>
        <input type="text" placeholder="query" />
        <button type="button">Run</button>
      </form>
    `
  },
  camera: {
    id: 'camera',
    title: 'Camera',
    defaultMode: 'fullscreen',
    overlays: {
      primaryKind: 'video',
      primary: 'video',
      legend: 'header.overlay'
    },
    render: () => `
      <video muted playsinline controls></video>
      ${legendHeader([
        ['stream', 'demo'],
        ['codec', 'h264'],
        ['fps', '30'],
        ['latency', '46ms'],
        ['drops', '0'],
        ['bitrate', '4mbps'],
        ['res', '1280x720'],
        ['audio', 'off']
      ])}
    `
  },
  docs: {
    id: 'docs',
    title: 'Docs',
    defaultMode: 'fullscreen',
    overlays: {
      primaryKind: 'docs',
      primary: 'article',
      legend: 'header.overlay'
    },
    render: () => `
      <article>
        <h2>Docs</h2>
        <p>This section is for guide text and onboarding notes.</p>
        <p>Switch layout mode at runtime to validate both shells.</p>
      </article>
      ${textHeader('Documentation', 'Guides and system notes')}
    `
  },
  terminal: {
    id: 'terminal',
    title: 'Terminal',
    defaultMode: 'calculator',
    overlays: {
      primaryKind: 'xterm',
      primary: 'pre',
      modeForm: 'form',
      legend: 'header.overlay'
    },
    render: () => `
      <pre>log> ready\n</pre>
      ${legendHeader([
        ['source', 'robot'],
        ['status', 'idle', 'terminal-status'],
        ['level', 'info'],
        ['tail', 'on'],
        ['errors', '0'],
        ['warn', '0'],
        ['info', '12'],
        ['mode', 'live']
      ])}
      <form>
        <button type="button">Clear</button>
        <button type="button">Tail</button>
        <button type="button">Retry</button>
        <button type="button">Pause</button>
        <button type="button">Resume</button>
        <button type="button">Errors</button>
        <button type="button">Info</button>
        <button type="button">Warn</button>
        <button type="button">Mode</button>
        <input type="text" placeholder="command" />
        <button type="button" data-action="terminal-send">Send</button>
      </form>
    `
  },
  settings: {
    id: 'settings',
    title: 'Settings',
    defaultMode: 'fullscreen',
    overlays: {
      primaryKind: 'button-list',
      primary: 'article',
      legend: 'header.overlay'
    },
    render: () => `
      <article>
        <button type="button">Toggle Chatlog</button>
        <button type="button">Use GPU</button>
        <button type="button">Reset Layout</button>
        <button type="button">Version:v1</button>
      </article>
      ${textHeader('Settings', 'Session and runtime controls')}
    `
  }
};

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
