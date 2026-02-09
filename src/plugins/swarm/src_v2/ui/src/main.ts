import './style.css'
import 'xterm/css/xterm.css'
import { SectionManager } from './util/section'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'

const sections = new SectionManager()

// 1. Register Sections
sections.register('s-demo', {
  containerId: 'demo-container',
  load: async () => {
    const { mountDemo } = await import('./components/demo')
    const container = document.getElementById('demo-container')!
    return mountDemo(container)
  }
})

sections.register('s-explorer', {
  containerId: 'app',
  load: async () => {
    initExplorer()
    return {
      dispose: () => {},
      setVisible: () => {}
    }
  }
})

sections.register('s-terminal', {
  containerId: 'app',
  load: async () => {
    initTerminal()
    return {
      dispose: () => {},
      setVisible: () => {}
    }
  }
})

sections.observe()
sections.load('s-demo')

// 2. Explorer Logic
function initExplorer() {
  const kvKey = document.getElementById('kv-key') as HTMLInputElement
  const kvVal = document.getElementById('kv-val') as HTMLInputElement
  const kvPut = document.getElementById('kv-put') as HTMLButtonElement
  const logMsg = document.getElementById('log-msg') as HTMLInputElement
  const logAppend = document.getElementById('log-append') as HTMLButtonElement

  kvPut.onclick = async () => {
    await fetch('/api/kv/put', {
      method: 'POST',
      body: JSON.stringify({ key: kvKey.value, value: kvVal.value })
    })
    kvKey.value = ''
    kvVal.value = ''
    updateExplorer()
  }

  logAppend.onclick = async () => {
    await fetch('/api/log/append', {
      method: 'POST',
      body: JSON.stringify({ msg: logMsg.value })
    })
    logMsg.value = ''
    updateExplorer()
  }

  setInterval(updateExplorer, 3000)
  updateExplorer()
}

async function updateExplorer() {
  try {
    const res = await fetch('/api/data')
    const data = await res.json()
    
    const kvList = document.getElementById('kv-list')!
    kvList.innerHTML = Object.entries(data.kv || {}).map(([k, v]) => `
      <div class="data-item">
        <span><strong>${k}:</strong> ${v}</span>
        <button onclick="deleteKV('${k}')">×</button>
      </div>
    `).join('')

    const logList = document.getElementById('log-list')!
    logList.innerHTML = (data.log || []).reverse().map((l: any) => `
      <div class="data-item">
        <span>${new Date(l.timestamp).toLocaleTimeString()} - ${JSON.stringify(l.data)}</span>
      </div>
    `).join('')
  } catch (e) {
    console.error('Update explorer failed', e)
  }
}

(window as any).deleteKV = async (key: string) => {
  await fetch('/api/kv/del', {
    method: 'POST',
    body: JSON.stringify({ key })
  })
  updateExplorer()
}

// 3. Terminal Logic
function initTerminal() {
  const term = new Terminal({
    theme: {
      background: '#000',
      foreground: '#00ff88'
    },
    cursorBlink: true,
    fontFamily: 'monospace'
  })
  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  
  const container = document.getElementById('terminal-container')!
  term.open(container)
  fitAddon.fit()

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(`${protocol}//${window.location.host}/terminal`)

  term.onData(data => {
    ws.send(data)
  })

  ws.onmessage = ev => {
    term.write(ev.data)
  }

  window.addEventListener('resize', () => fitAddon.fit())
  
  term.writeln('Welcome to Dialtone Swarm Terminal')
  term.writeln('Type "./dialtone.sh swarm help" to start')
  term.write('\r\n$ ')
}
