import './style.css'
import 'xterm/css/xterm.css'
import { SectionManager } from './util/section'

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
    const { initExplorer } = await import('./components/explorer')
    return initExplorer()
  }
})

sections.register('s-terminal', {
  containerId: 'app',
  load: async () => {
    const { initTerminal } = await import('./components/terminal')
    return initTerminal()
  }
})

sections.observe()
// Eager load first section
sections.load('s-demo')