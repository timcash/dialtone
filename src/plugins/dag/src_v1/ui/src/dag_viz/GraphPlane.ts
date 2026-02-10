import * as THREE from 'three'
import { GraphNode } from './GraphNode'
import { GraphLink } from './GraphLink'
import { LayoutEngine } from './LayoutEngine'

export class GraphPlane {
  id: string
  group: THREE.Group
  nodes: Map<string, GraphNode> = new Map()
  links: GraphLink[] = []
  yOffset: number

  constructor(id: string, yOffset: number) {
    this.id = id
    this.yOffset = yOffset
    this.group = new THREE.Group()
    this.group.position.y = yOffset
    this.group.userData = { type: 'layer', id }

    this.initPlane()
  }

  private initPlane(): void {
    const geometry = new THREE.PlaneGeometry(100, 100)
    const material = new THREE.MeshBasicMaterial({
      color: 0x0c1618,
      transparent: true,
      opacity: 0.18,
      side: THREE.DoubleSide
    })
    const plane = new THREE.Mesh(geometry, material)
    plane.rotation.x = -Math.PI / 2
    this.group.add(plane)

    const grid = new THREE.GridHelper(100, 50, 0x1f6f61, 0x0d1617)
    this.group.add(grid)
  }

  addNode(id: string, label: string): GraphNode {
    const node = new GraphNode(id, label)
    this.nodes.set(id, node)
    this.group.add(node.mesh)
    LayoutEngine.apply(this)
    return node
  }

  addLink(fromId: string, toId: string): void {
    const fromNode = this.nodes.get(fromId)
    const toNode = this.nodes.get(toId)
    if (fromNode && toNode) {
      const link = new GraphLink(fromNode, toNode)
      this.links.push(link)
      this.group.add(link.line)
      LayoutEngine.apply(this)
    }
  }
}
