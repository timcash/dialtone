import '../../../../../libs/ui/style.css'
import './style.css'
import { setupApp } from './dialtone-ui'

const { sections, menu } = setupApp({
  title: 'dialtone.dag',
  debug: true,
})

sections.register('dag-hero', {
  containerId: 'dag-hero-container',
  header: { visible: true, title: 'dialtone.dag', menuVisible: true },
  load: async () => {
    const { mountDagHero } = await import('./components/dag-hero')
    const container = document.getElementById('dag-hero-container')
    if (!container) throw new Error('dag-hero-container not found')
    return mountDagHero(container)
  },
})

sections.register('dag-docs', {
  containerId: 'dag-docs-container',
  header: { visible: false, menuVisible: false },
  load: async () => {
    const { mountDagDocs } = await import('./components/dag-docs')
    return mountDagDocs()
  },
})

sections.register('dag-layer-nest', {
  containerId: 'dag-layer-container',
  header: { visible: false, menuVisible: false },
  load: async () => {
    const { mountDagLayerNest } = await import('./components/dag-layer-nest')
    const container = document.getElementById('dag-layer-container')
    if (!container) throw new Error('dag-layer-container not found')
    return mountDagLayerNest(container)
  },
})

sections.observe()

menu.clear()
menu.addHeader('Navigation')
menu.addButton('Hero Toy', () => sections.navigateTo('dag-hero'), true)
menu.addButton('How It Works', () => sections.navigateTo('dag-docs'))
menu.addButton('Layer Nest', () => sections.navigateTo('dag-layer-nest'))

const orderedSections = ['dag-hero', 'dag-docs', 'dag-layer-nest']

const initialId = window.location.hash.slice(1) || 'dag-hero'
console.log(`[SectionManager] 🧭 INITIAL LOAD #${initialId}`)
sections.load(initialId).then(() => {
  const el = document.getElementById(initialId)
  if (el) {
    el.scrollIntoView({ behavior: 'auto', block: 'start' })
  }
})

window.addEventListener('keydown', (event) => {
  if (event.code !== 'Space' && event.keyCode !== 32) {
    return
  }
  event.preventDefault()
  const currentId = window.location.hash.slice(1) || 'dag-hero'
  const currentIndex = orderedSections.indexOf(currentId)
  const nextIndex = currentIndex >= 0 ? (currentIndex + 1) % orderedSections.length : 0
  sections.navigateTo(orderedSections[nextIndex])
})
