import * as THREE from 'three'

export class Stage {
  scene: THREE.Scene
  camera: THREE.PerspectiveCamera
  renderer: THREE.WebGLRenderer

  constructor(private container: HTMLElement) {
    this.scene = new THREE.Scene()
    this.scene.background = new THREE.Color(0x05070b)

    const { width, height } = this.getSize()
    this.camera = new THREE.PerspectiveCamera(60, width / height, 0.1, 1000)
    this.camera.position.set(0, 30, 60)
    this.camera.lookAt(0, 0, 0)

    this.renderer = new THREE.WebGLRenderer({ antialias: true, preserveDrawingBuffer: true })
    this.renderer.setPixelRatio(window.devicePixelRatio)
    this.renderer.setClearColor(0x05070b, 1)
    this.renderer.setSize(width, height, false)

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.5))
    const dirLight = new THREE.DirectionalLight(0xffffff, 1)
    dirLight.position.set(50, 50, 50)
    this.scene.add(dirLight)

    this.scene.add(new THREE.GridHelper(200, 20, 0x2c6b5d, 0x121c1d))

    this.container.appendChild(this.renderer.domElement)
  }

  private getSize() {
    const rect = this.container.getBoundingClientRect()
    return { width: Math.max(rect.width, 1), height: Math.max(rect.height, 1) }
  }

  resize(): void {
    const { width, height } = this.getSize()
    this.camera.aspect = width / height
    this.camera.updateProjectionMatrix()
    this.renderer.setSize(width, height, false)
  }

  render(): void {
    this.renderer.render(this.scene, this.camera)
  }
}
