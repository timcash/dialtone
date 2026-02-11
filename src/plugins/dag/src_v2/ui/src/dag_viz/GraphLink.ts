import * as THREE from 'three'
import { GraphNode } from './GraphNode'

export class GraphLink {
  from: GraphNode
  to: GraphNode
  line: THREE.Line

  constructor(from: GraphNode, to: GraphNode) {
    this.from = from
    this.to = to

    const geometry = new THREE.BufferGeometry()
    const material = new THREE.LineBasicMaterial({ color: 0x66b7ff, transparent: true, opacity: 0.55 })
    this.line = new THREE.Line(geometry, material)

    this.update()
  }

  update(): void {
    const start = this.from.mesh.position
    const end = this.to.mesh.position
    const midX = (start.x + end.x) / 2

    const curve = new THREE.CubicBezierCurve3(
      start,
      new THREE.Vector3(midX, start.y + 2, start.z),
      new THREE.Vector3(midX, end.y + 2, end.z),
      end
    )

    const points = curve.getPoints(20)
    this.line.geometry.setFromPoints(points)
  }
}
