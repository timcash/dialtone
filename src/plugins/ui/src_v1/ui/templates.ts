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
    <header class="overlay text" aria-label="Text Header">
      <h1>${title}</h1>
      <h3>${subtitle}</h3>
    </header>
  `;
}

function legendHeader(pairs: Array<[string, string, string?]>): string {
  const cells = pairs
    .map(
      ([label, value, className]) => `
        <div>
          <dt>${label}</dt>
          <dd${className ? ` class="${className}"` : ''}>${value}</dd>
        </div>
      `
    )
    .join('');
  return `
    <header class="overlay legend" aria-label="Legend Header">
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
      <canvas aria-label="Hero Canvas"></canvas>
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
        <button type="button" aria-label="Hero Arm">Arm</button>
        <button type="button" aria-label="Hero Disarm">Disarm</button>
        <button type="button" aria-label="Hero Manual">Manual</button>
        <button type="button" aria-label="Hero Guided">Guided</button>
        <button type="button" aria-label="Hero Pause">Pause</button>
        <button type="button" aria-label="Hero Resume">Resume</button>
        <button type="button" aria-label="Hero Home">Home</button>
        <button type="button" aria-label="Hero Land">Land</button>
        <button type="button" aria-label="Hero Mode">Mode</button>
        <input type="text" aria-label="Hero Command Input" placeholder="command" />
        <button type="button" aria-label="Hero Send">Send</button>
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
      <canvas aria-label="Three Fullscreen Canvas"></canvas>
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
      <canvas aria-label="Three Calculator Canvas"></canvas>
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
        <button type="button" aria-label="Three Select">Select</button>
        <button type="button" aria-label="Three Add" class="three-add">Add</button>
        <button type="button" aria-label="Three Link">Link</button>
        <button type="button" aria-label="Three Delete">Delete</button>
        <button type="button" aria-label="Three Group">Group</button>
        <button type="button" aria-label="Three Split">Split</button>
        <button type="button" aria-label="Three Frame">Frame</button>
        <button type="button" aria-label="Three Labels">Labels</button>
        <button type="button" aria-label="Three Mode">Mode</button>
        <input type="text" aria-label="Three Input" placeholder="label" />
        <button type="button" aria-label="Three Apply">Apply</button>
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
      <table aria-label="Table Underlay">
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
        <button type="button" aria-label="Table Refresh" class="table-refresh">Refresh</button>
        <button type="button" aria-label="Table Filter">Filter</button>
        <button type="button" aria-label="Table Export">Export</button>
        <button type="button" aria-label="Table Sort">Sort</button>
        <button type="button" aria-label="Table Focus">Focus</button>
        <button type="button" aria-label="Table Details">Details</button>
        <button type="button" aria-label="Table Diff">Diff</button>
        <button type="button" aria-label="Table Tail">Tail</button>
        <button type="button" aria-label="Table Mode">Mode</button>
        <input type="text" aria-label="Table Query Input" placeholder="query" />
        <button type="button" aria-label="Table Run">Run</button>
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
      <video aria-label="Camera Video" muted playsinline controls></video>
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
      <article aria-label="Docs Underlay">
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
      <pre aria-label="Terminal Underlay">log> ready\n</pre>
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
        <button type="button" aria-label="Terminal Clear">Clear</button>
        <button type="button" aria-label="Terminal Tail">Tail</button>
        <button type="button" aria-label="Terminal Retry">Retry</button>
        <button type="button" aria-label="Terminal Pause">Pause</button>
        <button type="button" aria-label="Terminal Resume">Resume</button>
        <button type="button" aria-label="Terminal Errors">Errors</button>
        <button type="button" aria-label="Terminal Info">Info</button>
        <button type="button" aria-label="Terminal Warn">Warn</button>
        <button type="button" aria-label="Terminal Mode">Mode</button>
        <input type="text" aria-label="Terminal Input" placeholder="command" />
        <button type="button" aria-label="Terminal Send" class="terminal-send">Send</button>
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
      <article aria-label="Settings Underlay">
        <button type="button" aria-label="Settings Toggle Chatlog">Toggle Chatlog</button>
        <button type="button" aria-label="Settings Use GPU">Use GPU</button>
        <button type="button" aria-label="Settings Reset Layout">Reset Layout</button>
        <button type="button" aria-label="Settings Version">Version:v1</button>
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
