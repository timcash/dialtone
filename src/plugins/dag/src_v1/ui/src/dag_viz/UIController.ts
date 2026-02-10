import { DagApp } from './app'

export class UIController {
  breadcrumbElement: HTMLElement | null = null
  minimapCanvas: HTMLCanvasElement | null = null
  leftMenu: HTMLElement | null = null
  rightMenu: HTMLElement | null = null

  constructor(private app: DagApp, private uiLayer: HTMLElement) {
    this.initOverlay()
  }

  private initOverlay(): void {
    this.breadcrumbElement = document.createElement('div')
    this.breadcrumbElement.className = 'dag-breadcrumb'
    this.breadcrumbElement.innerText = 'Root'
    this.uiLayer.appendChild(this.breadcrumbElement)

    this.minimapCanvas = document.createElement('canvas')
    this.minimapCanvas.className = 'dag-minimap'
    this.minimapCanvas.width = 160
    this.minimapCanvas.height = 160
    this.uiLayer.appendChild(this.minimapCanvas)

    this.leftMenu = this.createThumbMenu('left', ['Add Node', 'Clear'])
    this.rightMenu = this.createThumbMenu('right', ['Reset Cam', 'Help'])

    this.uiLayer.appendChild(this.leftMenu)
    this.uiLayer.appendChild(this.rightMenu)
  }

  private createThumbMenu(side: 'left' | 'right', items: string[]): HTMLElement {
    const wrapper = document.createElement('div')
    wrapper.className = `dag-thumb-wrapper ${side}`

    const button = document.createElement('button')
    button.className = 'dag-thumb-main'
    button.innerText = side === 'left' ? 'EDIT' : 'HELP'

    const stack = document.createElement('div')
    stack.className = 'dag-thumb-stack'

    items.forEach(text => {
      const subBtn = document.createElement('div')
      subBtn.className = 'dag-thumb-sub'
      subBtn.innerText = text
      stack.appendChild(subBtn)
    })

    const toggleMenu = () => {
      wrapper.classList.toggle('is-open')
    }

    button.addEventListener('click', toggleMenu)

    wrapper.appendChild(button)
    wrapper.appendChild(stack)
    return wrapper
  }

  updateMinimap(): void {
    if (!this.minimapCanvas) return
    const ctx = this.minimapCanvas.getContext('2d')
    if (!ctx) return

    ctx.clearRect(0, 0, 160, 160)
    ctx.fillStyle = '#7bf2d8'

    this.app.rootPlane.nodes.forEach(node => {
      const x = 80 + node.mesh.position.x * 2
      const y = 80 + node.mesh.position.z * 2
      ctx.fillRect(x - 2, y - 2, 4, 4)
    })
  }

  updateBreadcrumbs(): void {
    if (!this.breadcrumbElement) return
    const path = this.app.navigator.path
    this.breadcrumbElement.innerText = ['Root', ...path].join(' > ')
  }
}
