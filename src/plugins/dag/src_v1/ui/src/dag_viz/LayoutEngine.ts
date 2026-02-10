import { GraphPlane } from './GraphPlane'

export class LayoutEngine {
  static apply(plane: GraphPlane): void {
    plane.nodes.forEach(node => (node.rank = 0))

    let changed = true
    while (changed) {
      changed = false
      plane.links.forEach(link => {
        if (link.to.rank <= link.from.rank) {
          link.to.rank = link.from.rank + 1
          changed = true
        }
      })
    }

    const rankCounts: Map<number, number> = new Map()
    plane.nodes.forEach(node => {
      const count = rankCounts.get(node.rank) || 0
      node.mesh.position.x = node.rank * 10
      node.mesh.position.z = count * 5
      rankCounts.set(node.rank, count + 1)
    })

    plane.links.forEach(link => link.update())

    plane.nodes.forEach(node => {
      if (node.subPlane) {
        LayoutEngine.apply(node.subPlane)
      }
    })
  }
}
