import './style.css'
import { SectionManager } from './util/section'
import { Menu } from './util/menu'

const sections = new SectionManager({ debug: false })

sections.register('dag-hero', {
  containerId: 'dag-hero-container',
  header: { visible: true, title: 'dialtone.dag' },
  menu: { visible: true },
  load: async () => {
    const { mountDagHero } = await import('./components/dag-hero')
    const container = document.getElementById('dag-hero-container')
    if (!container) throw new Error('dag-hero-container not found')
    return mountDagHero(container)
  }
})

sections.register('dag-docs', {
  containerId: 'dag-docs-container',
  header: { visible: false },
  menu: { visible: false },
  load: async () => {
    const { mountDagDocs } = await import('./components/dag-docs')
    return mountDagDocs()
  }
})

sections.register('dag-layer-nest', {
  containerId: 'dag-layer-container',
  header: { visible: false },
  menu: { visible: false },
  load: async () => {
    const { mountDagLayerNest } = await import('./components/dag-layer-nest')
    const container = document.getElementById('dag-layer-container')
    if (!container) throw new Error('dag-layer-container not found')
    return mountDagLayerNest(container)
  }
})

sections.observe()

const menu = Menu.getInstance()
menu.clear()
menu.addHeader('Navigation')
menu.addButton('Hero Toy', () => loadSection('dag-hero', true), true)
menu.addButton('How It Works', () => loadSection('dag-docs', true))
menu.addButton('Layer Nest', () => loadSection('dag-layer-nest', true))

const initialHash = window.location.hash.slice(1)
let isProgrammaticScroll = false
let programmaticScrollTimeout: number | null = null

const loadSection = (id: string, smooth = false) => {
  if (id && document.getElementById(id)?.classList.contains('snap-slide')) {
    sections.eagerLoad(id)
    const el = document.getElementById(id)
    if (el) {
      isProgrammaticScroll = true
      if (programmaticScrollTimeout) clearTimeout(programmaticScrollTimeout)

      requestAnimationFrame(() => {
        el.scrollIntoView({ behavior: smooth ? 'smooth' : 'auto', block: 'start' })
        programmaticScrollTimeout = window.setTimeout(() => {
          isProgrammaticScroll = false
          programmaticScrollTimeout = null
        }, 3000)
      })
    }
    return true
  }
  return false
}

;(window as any).navigateTo = (id: string) => loadSection(id, true)

if (!loadSection(initialHash)) {
  sections.eagerLoad('dag-hero')
}

window.addEventListener('hashchange', () => {
  const hash = window.location.hash.slice(1)
  loadSection(hash, true)
})

const slides = document.querySelectorAll('.snap-slide[data-subtitle]')
const allSlides = document.querySelectorAll('.snap-slide')

const hashObserver = new IntersectionObserver(
  (entries) => {
    if (isProgrammaticScroll) return
    let best: { id: string; ratio: number } | null = null
    for (const entry of entries) {
      if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
        const id = (entry.target as HTMLElement).id
        if (id && (!best || entry.intersectionRatio > best.ratio)) {
          best = { id, ratio: entry.intersectionRatio }
        }
      }
    }
    if (best && location.hash.slice(1) !== best.id) {
      history.replaceState(null, '', '#' + best.id)
    }
  },
  { threshold: [0.5, 0.75, 1] }
)

setTimeout(() => {
  allSlides.forEach((el) => hashObserver.observe(el))
}, 1000)

const marketingObserver = new IntersectionObserver(
  (entries) => {
    entries.forEach((entry) => {
      entry.target.classList.toggle('is-visible', entry.isIntersecting)
    })
  },
  { threshold: 0.45 }
)

slides.forEach((slide) => marketingObserver.observe(slide))

window.addEventListener('keydown', (e) => {
  if (e.code === 'Space' || e.keyCode === 32) {
    e.preventDefault()
    const slideList = Array.from(document.querySelectorAll('.snap-slide'))
    const currentSlideIndex = slideList.findIndex((slide) => {
      const rect = slide.getBoundingClientRect()
      return rect.top >= -10 && rect.top <= 10
    })

    const nextIndex = (currentSlideIndex + 1) % slideList.length
    slideList[nextIndex].scrollIntoView({ behavior: 'smooth' })
  }
})
