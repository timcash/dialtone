import * as THREE from 'three'
import { GraphPlane } from './GraphPlane'

export class GraphNode {
  id: string
  label: string
  mesh: THREE.Mesh
  subPlane: GraphPlane | null = null
  rank = 0

  constructor(id: string, label: string) {
    this.id = id
    this.label = label

    const geometry = new THREE.BoxGeometry(4, 0.5, 2)
    const material = new THREE.MeshPhongMaterial({
      color: 0x1b2b2b,
      emissive: 0x0f1d1d,
      emissiveIntensity: 0.6
    })
    this.mesh = new THREE.Mesh(geometry, material)
    this.mesh.userData = { id, type: 'node' }

    const edges = new THREE.EdgesGeometry(geometry)
    const lineMaterial = new THREE.LineBasicMaterial({ color: 0x7bf2d8, transparent: true, opacity: 0.55 })
    const wireframe = new THREE.LineSegments(edges, lineMaterial)
    this.mesh.add(wireframe)

    this.createLabel(label)
  }

  private createLabel(text: string): void {
    const canvas = document.createElement('canvas')
    canvas.width = 256
    canvas.height = 128
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    ctx.fillStyle = '#04070b'
    ctx.fillRect(0, 0, 256, 128)
    ctx.font = 'bold 38px Space Grotesk, sans-serif'
    ctx.fillStyle = '#e6f7ff'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    ctx.fillText(text, 128, 64)

    const texture = new THREE.CanvasTexture(canvas)
    const geometry = new THREE.PlaneGeometry(3.6, 1.6)
    const material = new THREE.MeshBasicMaterial({ map: texture, transparent: true })
    const labelMesh = new THREE.Mesh(geometry, material)

    labelMesh.position.y = 0.26
    labelMesh.rotation.x = -Math.PI / 2
    this.mesh.add(labelMesh)
  }

  setHover(active: boolean): void {
    const mat = this.mesh.material as THREE.MeshPhongMaterial
    const wireframe = this.mesh.children.find(c => c instanceof THREE.LineSegments) as THREE.LineSegments | undefined
    const borderMat = wireframe?.material as THREE.LineBasicMaterial | undefined

    if (active) {
      mat.color.setHex(0x0b1f22)
      mat.emissive.setHex(0x0088ff)
      mat.emissiveIntensity = 2.0
      if (borderMat) {
        borderMat.color.setHex(0x66f0ff)
        borderMat.opacity = 1.0
      }
      if (this.subPlane) this.subPlane.group.visible = true
    } else {
      mat.color.setHex(0x1b2b2b)
      mat.emissive.setHex(0x0f1d1d)
      mat.emissiveIntensity = 0.6
      if (borderMat) {
        borderMat.color.setHex(0x7bf2d8)
        borderMat.opacity = 0.55
      }
      if (this.subPlane) this.subPlane.group.visible = false
    }
  }

  ensureSubLayer(): GraphPlane {
    if (!this.subPlane) {
      this.subPlane = new GraphPlane(`${this.id}_sub`, -20)
      this.subPlane.group.visible = false
      this.mesh.add(this.subPlane.group)
    }
    return this.subPlane
  }
}
