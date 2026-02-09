import './style.css'
import { SectionManager } from './util/section'
import { Menu } from './util/menu'

const sections = new SectionManager()
Menu.getInstance()

// 1. Register Sections
sections.register('s-viz', {
  containerId: 'viz-container',
  load: async () => {
    const { mountNixViz } = await import('./components/nix-viz')
    const container = document.getElementById('viz-container')!
    return mountNixViz(container)
  }
})

sections.register('s-nixtable', {
  containerId: 's-nixtable',
  header: { visible: false }, // Hide HUD/Header in table view
  load: async () => {
    const stopInterval = initManager()
    return {
      dispose: () => { stopInterval() },
      setVisible: (v: boolean) => {
          const el = document.getElementById('s-nixtable');
          if (el) el.style.visibility = v ? 'visible' : 'hidden';
      }
    }
  }
})

sections.observe()

// 2. Navigation & Hash Management
const loadSection = (id: string, smooth = true) => {
    const el = document.getElementById(id);
    if (el && el.classList.contains('snap-slide')) {
        sections.load(id); 
        el.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: 'start' });
        return true;
    }
    return false;
};

const initialHash = window.location.hash.slice(1) || 's-viz';
setTimeout(() => loadSection(initialHash, false), 100);

window.addEventListener('hashchange', () => {
    loadSection(window.location.hash.slice(1), true);
});

// Marketing fade-in
const marketingObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        entry.target.classList.toggle('is-visible', entry.isIntersecting);
    });
}, { threshold: 0.45 });

document.querySelectorAll('.snap-slide').forEach(slide => marketingObserver.observe(slide));

// 3. Manager Logic
function initManager() {
  const startBtn = document.getElementById('start-node') as HTMLButtonElement
  if (!startBtn) return () => {}

  startBtn.onclick = async () => {
    console.log('[NIX] Spawning new node...')
    try {
        const res = await fetch('/api/processes', { method: 'POST' })
        const data = await res.json()
        console.log('[NIX] Successfully spawned ' + data.id)
    } catch (e) {
        console.error('[NIX] Spawn failed', e)
    }
    updateSpreadsheet()
  }

  const interval = setInterval(updateSpreadsheet, 2000)
  updateSpreadsheet()
  
  return () => clearInterval(interval)
}

async function updateSpreadsheet() {
  try {
    const res = await fetch('/api/processes')
    const procs = await res.json()
    const tbody = document.getElementById('node-rows')!
    if (!tbody) return

    if (!procs || !Array.isArray(procs) || procs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="4" style="padding: 20px; text-align: center; opacity: 0.5;">No active nodes found</td></tr>';
        return;
    }

    tbody.innerHTML = procs.map((p: any) => {
      const lastLog = p.logs && p.logs.length > 0 ? p.logs[p.logs.length - 1] : 'Waiting for logs...';
      const statusColor = p.status === 'running' ? '#004422' : '#440000';
      
      return '<tr class="node-row" id="' + p.id + '" data-status="' + p.status + '" style="border-bottom: 1px solid #222;">' +
        '<td style="padding: 12px; font-weight: bold; color: #00ff88;">' + p.id + '</td>' +
        '<td style="padding: 12px;">' +
          '<span class="status-badge" data-status-text="' + p.status + '" style="padding: 2px 6px; border-radius: 3px; background: ' + statusColor + '; font-size: 11px;">' + 
            p.status.toUpperCase() + 
          '</span>' +
        '</td>' +
        '<td class="node-logs" style="padding: 12px; color: #888; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + lastLog + '</td>' +
        '<td style="padding: 12px; text-align: right;">' +
          '<button class="stop-btn" aria-label="Stop Node ' + p.id + '" onclick="stopNode(\'' + p.id + '\')" style="background: #331111; color: #ff4444; border: 1px solid #552222; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px;">STOP</button>' +
        '</td>' +
      '</tr>';
    }).join('')
  } catch (e) {
    console.error('[NIX] Update spreadsheet failed', e)
  }
}

(window as any).stopNode = async (id: string) => {
  console.log('[NIX] Requesting stop for node ' + id)
  try {
      await fetch('/api/stop?id=' + id)
      console.log('[NIX] Stop command acknowledged for ' + id)
  } catch (e) {
      console.error('[NIX] Failed to stop ' + id, e)
  }
  updateSpreadsheet()
}
