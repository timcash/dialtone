import * as THREE from 'three'

export class Navigator {
  camera: THREE.PerspectiveCamera
  path: string[] = []
  history: string[] = []
  currentY = 0

  constructor(camera: THREE.PerspectiveCamera) {
    this.camera = camera
  }

  dive(node: any): void {
    this.path.push(node.id)
    this.history = []

    const worldPos = new THREE.Vector3()
    node.mesh.getWorldPosition(worldPos)

    this.camera.position.set(worldPos.x, worldPos.y - 10, worldPos.z + 40)
    this.camera.lookAt(worldPos.x, worldPos.y - 20, worldPos.z)
    this.currentY = worldPos.y - 20
  }

  scrub(delta: number): void {
    if (delta < 0 && this.path.length > 0) {
      const popped = this.path.pop()
      if (popped) this.history.push(popped)

      this.camera.position.set(0, 30, 60)
      this.camera.lookAt(0, 0, 0)
      this.currentY = 0
    } else if (delta > 0 && this.history.length > 0) {
      const nodeId = this.history.pop()
      if (nodeId) {
        this.path.push(nodeId)
        this.camera.position.set(0, -10, 40)
        this.camera.lookAt(0, -20, 0)
        this.currentY = -20
      }
    }
  }
}
