import './style.css'
import { SectionManager } from './util/section'
import { Menu } from './util/menu'

const sections = new SectionManager()
Menu.getInstance()

// 1. Register Sections
sections.register('nix-hero', {
  containerId: 'viz-container',
  load: async () => {
    const { mountNixViz } = await import('./components/nix-viz')
    const container = document.getElementById('viz-container')!
    return mountNixViz(container)
  }
})

sections.register('nix-docs', {
  containerId: 'nix-docs',
  load: async () => {
    return {
      dispose: () => {},
      setVisible: (v: boolean) => {
          const el = document.getElementById('nix-docs');
          if (el) {
              el.style.visibility = v ? 'visible' : 'hidden';
              el.style.opacity = v ? '1' : '0';
          }
      }
    }
  }
})

sections.register('nix-table', {
  containerId: 'nix-table',
  header: { 
    visible: false,
    menuVisible: false // Hide global menu too
  },
  load: async () => {
    const stopInterval = initManager()
    return {
      dispose: () => { stopInterval() },
      setVisible: (v: boolean) => {
          console.log('[NIX] nix-table setVisible:', v);
          const el = document.getElementById('nix-table');
          if (el) {
              el.style.visibility = 'visible';
              el.style.opacity = '1';
          }
      }
    }
  }
})

sections.observe()

// 2. Navigation & Hash Management
let isProgrammaticScroll = false;
let programmaticScrollTimeout: number | null = null;

(window as any).navigateTo = (id: string, smooth = true) => {
    console.log(`[main] 🧭 Navigating to: #${id} (smooth: ${smooth})`);
    const el = document.getElementById(id);
    if (el && el.classList.contains('snap-slide')) {
        sections.load(id); 
        
        isProgrammaticScroll = true;
        if (programmaticScrollTimeout) clearTimeout(programmaticScrollTimeout);

        el.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: 'start' });
        
        // Force is-visible class for programmatic navigation
        document.querySelectorAll('.snap-slide').forEach(s => s.classList.remove('is-visible'));
        el.classList.add('is-visible');

        programmaticScrollTimeout = window.setTimeout(() => {
            console.log(`[main] ✅ Programmatic scroll settled for #${id}`);
            isProgrammaticScroll = false;
            programmaticScrollTimeout = null;
        }, 2000);

        return true;
    }
    return false;
};

const initialHash = window.location.hash.slice(1) || 'nix-hero';
setTimeout(() => (window as any).navigateTo(initialHash, false), 100);

window.addEventListener('hashchange', () => {
    const hash = window.location.hash.slice(1);
    if (hash) {
        (window as any).navigateTo(hash, true);
    }
});

// Update URL hash when scroll brings a section into view
const allSlides = document.querySelectorAll('.snap-slide');
const hashObserver = new IntersectionObserver(
    (entries) => {
        if (isProgrammaticScroll) return;
        let best: { id: string; ratio: number } | null = null;
        for (const entry of entries) {
            if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
                const id = (entry.target as HTMLElement).id;
                if (id && (!best || entry.intersectionRatio > best.ratio)) {
                    best = { id, ratio: entry.intersectionRatio };
                }
            }
        }
        if (best && window.location.hash.slice(1) !== best.id) {
            console.log(`[main] 🔃 Observer updating hash to #${best.id}`);
            history.replaceState(null, '', '#' + best.id);
        }
    },
    { threshold: [0.5, 0.75, 1] }
);

setTimeout(() => {
    allSlides.forEach((el) => hashObserver.observe(el));
}, 1000);

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

  startBtn.addEventListener('click', async () => {
    console.log('[NIX] Spawning new node...')
    try {
        const res = await fetch('/api/processes', { method: 'POST' })
        const data = await res.json()
        console.log('[NIX] Successfully spawned ' + data.id)
    } catch (e) {
        console.error('[NIX] Spawn failed', e)
    }
    updateSpreadsheet()
  })

  const interval = setInterval(updateSpreadsheet, 1000)
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
        tbody.innerHTML = '<tr><td colspan="6" style="padding: 20px; text-align: center; opacity: 0.5;">No active nodes found</td></tr>';
        return;
    }

    // Sort by name (ID) to keep them stable
    procs.sort((a: any, b: any) => a.id.localeCompare(b.id, undefined, { numeric: true, sensitivity: 'base' }));

    console.log('[NIX] Processes updated:', procs.map((p: any) => `${p.id}:${p.status}`).join(', '))

    tbody.innerHTML = procs.map((p: any) => {
      const lastLog = p.logs && p.logs.length > 0 ? p.logs[p.logs.length - 1] : 'Waiting for logs...';
      const statusColor = p.status === 'running' ? '#004422' : '#440000';
      
      return '<tr class="node-row" id="' + p.id + '" data-status="' + p.status + '" style="border-bottom: 1px solid #222;">' +
        '<td style="padding: 12px; font-weight: bold; color: #00ff88;">' + p.id + '</td>' +
        '<td style="padding: 12px; color: #aaa;">' + (p.pid || '-') + '</td>' +
        '<td style="padding: 12px;">' +
          '<span class="status-badge" data-status-text="' + p.status + '" style="padding: 2px 6px; border-radius: 3px; background: ' + statusColor + '; font-size: 11px;">' + 
            p.status.toUpperCase() + 
          '</span>' +
        '</td>' +
        '<td style="padding: 12px; color: #aaa;">' + (p.start_time || '-') + '</td>' +
        '<td class="node-logs" style="padding: 12px; color: #888; max-width: 400px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">' + lastLog + '</td>' +
        '<td style="padding: 12px; text-align: right;">' +
          '<button class="stop-btn" data-id="' + p.id + '" aria-label="Stop Node ' + p.id + '" style="background: #331111; color: #ff4444; border: 1px solid #552222; padding: 4px 8px; border-radius: 4px; cursor: pointer; font-size: 11px;">STOP</button>' +
        '</td>' +
      '</tr>';
    }).join('')

    // Attach event listeners after rendering
    tbody.querySelectorAll('.stop-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            console.log('[NIX] Stop button clicked for:', (btn as HTMLElement).dataset.id)
            const id = (btn as HTMLElement).dataset.id;
            if (id) stopNode(id);
        });
    });
  } catch (e) {
    console.error('[NIX] Update spreadsheet failed', e)
  }
}

async function stopNode(id: string) {
  console.log('[NIX] Requesting stop for node ' + id)
  try {
      const res = await fetch('/api/stop?id=' + id)
      if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`)
      console.log('[NIX] Stop command acknowledged for ' + id)
  } catch (e) {
      console.error('[NIX] Failed to stop ' + id, e)
  }
  updateSpreadsheet()
}

// Keep global for backwards compatibility if needed, but we use internal stopNode now
(window as any).stopNode = stopNode;