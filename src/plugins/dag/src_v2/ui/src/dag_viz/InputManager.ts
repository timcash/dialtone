import * as THREE from 'three'
import type { DagApp } from './app'

export class InputManager {
  private raycaster = new THREE.Raycaster()
  private mouse = new THREE.Vector2()
  private onMouseMoveBound = (e: MouseEvent) => this.onMouseMove(e)
  private onClickBound = (e: MouseEvent) => this.onClick(e)
  private onDoubleClickBound = (e: MouseEvent) => this.onDoubleClick(e)
  private onWheelBound = (e: WheelEvent) => this.onWheel(e)

  constructor(private app: DagApp, private container: HTMLElement) {
    this.initListeners()
  }

  dispose() {
    this.container.removeEventListener('mousemove', this.onMouseMoveBound)
    this.container.removeEventListener('click', this.onClickBound)
    this.container.removeEventListener('dblclick', this.onDoubleClickBound)
    this.container.removeEventListener('wheel', this.onWheelBound)
  }

  private initListeners(): void {
    this.container.addEventListener('mousemove', this.onMouseMoveBound)
    this.container.addEventListener('click', this.onClickBound)
    this.container.addEventListener('dblclick', this.onDoubleClickBound)
    this.container.addEventListener('wheel', this.onWheelBound, { passive: false })
  }

  private setMouseFromEvent(e: MouseEvent | WheelEvent): void {
    const rect = this.container.getBoundingClientRect()
    this.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1
    this.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1
  }

  private onDoubleClick(e: MouseEvent): void {
    this.setMouseFromEvent(e)
    this.raycaster.setFromCamera(this.mouse, this.app.stage.camera)

    const ground = this.app.currentPlane.group.children.find(c => (c as THREE.Mesh).geometry?.type === 'PlaneGeometry')
    if (!ground) return

    const intersects = this.raycaster.intersectObject(ground)
    if (intersects.length > 0) {
      const pt = intersects[0].point
      const id = `node_${Date.now()}`
      const node = this.app.currentPlane.addNode(id, 'New Node')
      const localPt = this.app.currentPlane.group.worldToLocal(pt.clone())
      node.mesh.position.set(localPt.x, 0, localPt.z)
    }
  }

  private onMouseMove(e: MouseEvent): void {
    this.setMouseFromEvent(e)
    this.raycaster.setFromCamera(this.mouse, this.app.stage.camera)

    const allNodes: THREE.Object3D[] = []
    const collectNodes = (plane: any) => {
      plane.nodes.forEach((node: any) => {
        allNodes.push(node.mesh)
        if (node.subPlane) collectNodes(node.subPlane)
      })
    }
    collectNodes(this.app.rootPlane)

    const intersects = this.raycaster.intersectObjects(allNodes, true)

    const activeHoverIds = new Set<string>()

    if (intersects.length > 0) {
      let obj: THREE.Object3D | null = intersects[0].object
      while (obj) {
        if (obj.userData && obj.userData.type === 'node') {
          const nodeId = obj.userData.id
          activeHoverIds.add(nodeId)

          let parent = obj.parent
          while (parent) {
            if (parent.userData && parent.userData.type === 'node') {
              activeHoverIds.add(parent.userData.id)
            }
            parent = parent.parent
          }
          break
        }
        obj = obj.parent
      }
    }

    const applyHover = (plane: any) => {
      plane.nodes.forEach((node: any) => {
        node.setHover(activeHoverIds.has(node.id))
        if (node.subPlane) applyHover(node.subPlane)
      })
    }
    applyHover(this.app.rootPlane)
  }

  private onClick(e: MouseEvent): void {
    this.setMouseFromEvent(e)
    this.raycaster.setFromCamera(this.mouse, this.app.stage.camera)

    const allNodes: any[] = []
    const collectNodes = (plane: any) => {
      plane.nodes.forEach((node: any) => {
        allNodes.push(node)
        if (node.subPlane) collectNodes(node.subPlane)
      })
    }
    collectNodes(this.app.rootPlane)

    const intersects = this.raycaster.intersectObjects(allNodes.map(n => n.mesh), true)

    if (intersects.length > 0) {
      let obj: THREE.Object3D | null = intersects[0].object
      while (obj) {
        if (obj.userData && obj.userData.type === 'node') {
          const nodeId = obj.userData.id
          const node = allNodes.find(n => n.id === nodeId)
          if (node) {
            this.app.navigator.dive(node)
          }
          break
        }
        obj = obj.parent
      }
    }
  }

  private onWheel(e: WheelEvent): void {
    e.preventDefault()
    this.app.navigator.scrub(e.deltaY)
  }
}
