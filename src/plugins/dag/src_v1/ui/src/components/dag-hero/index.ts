import * as THREE from 'three'
import type { VisualizationControl } from '../../util/section'

class DagHeroToy {
  private scene = new THREE.Scene()
  private camera: THREE.PerspectiveCamera
  private renderer: THREE.WebGLRenderer
  private frameId = 0
  private isVisible = true
  private nodes: THREE.Mesh[] = []

  constructor(private container: HTMLElement) {
    this.camera = new THREE.PerspectiveCamera(45, 1, 0.1, 100)
    this.camera.position.set(0, 1.5, 6)

    this.renderer = new THREE.WebGLRenderer({ antialias: true })
    this.renderer.setPixelRatio(window.devicePixelRatio)
    this.renderer.setClearColor(0x05070b, 1)
    this.container.appendChild(this.renderer.domElement)

    const ambient = new THREE.AmbientLight(0x88f2d8, 0.4)
    const key = new THREE.DirectionalLight(0xffffff, 0.9)
    key.position.set(5, 6, 5)
    this.scene.add(ambient, key)

    const grid = new THREE.GridHelper(30, 30, 0x113a36, 0x0a1416)
    grid.position.y = -1.5
    this.scene.add(grid)

    const coreGeo = new THREE.TorusKnotGeometry(0.9, 0.22, 120, 16)
    const coreMat = new THREE.MeshStandardMaterial({ color: 0x7bf2d8, metalness: 0.2, roughness: 0.2 })
    const core = new THREE.Mesh(coreGeo, coreMat)
    this.scene.add(core)
    this.nodes.push(core)

    const orbGeo = new THREE.SphereGeometry(0.14, 24, 24)
    const orbMat = new THREE.MeshStandardMaterial({ color: 0x66b7ff, emissive: 0x0b2144 })
    for (let i = 0; i < 10; i++) {
      const orb = new THREE.Mesh(orbGeo, orbMat)
      orb.position.set(Math.cos(i) * 2.4, (i % 3) * 0.4 - 0.4, Math.sin(i) * 2.4)
      this.scene.add(orb)
      this.nodes.push(orb)
    }

    this.resize()
    this.animate()
  }

  resize() {
    const { clientWidth, clientHeight } = this.container
    if (clientWidth === 0 || clientHeight === 0) return
    this.camera.aspect = clientWidth / clientHeight
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(clientWidth, clientHeight, false)
  }

  setVisible(visible: boolean) {
    this.isVisible = visible
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate)
    if (!this.isVisible) return

    this.nodes.forEach((node, i) => {
      node.rotation.x += 0.004 + i * 0.0004
      node.rotation.y += 0.006 + i * 0.0003
    })

    this.renderer.render(this.scene, this.camera)
  }

  dispose() {
    cancelAnimationFrame(this.frameId)
    this.renderer.dispose()
    if (this.renderer.domElement.parentElement) {
      this.renderer.domElement.parentElement.removeChild(this.renderer.domElement)
    }
  }
}

export function mountDagHero(container: HTMLElement): VisualizationControl {
  const toy = new DagHeroToy(container)
  const onResize = () => toy.resize()
  window.addEventListener('resize', onResize)

  return {
    dispose: () => {
      window.removeEventListener('resize', onResize)
      toy.dispose()
    },
    setVisible: (v: boolean) => toy.setVisible(v)
  }
}
