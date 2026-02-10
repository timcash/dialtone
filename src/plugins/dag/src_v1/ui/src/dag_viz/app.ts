import * as THREE from 'three'
import { Stage } from './Stage'
import { Navigator } from './Navigator'
import { InputManager } from './InputManager'
import { UIController } from './UIController'
import { GraphPlane } from './GraphPlane'
import { LayoutEngine } from './LayoutEngine'

export class DagApp {
  stage: Stage
  navigator: Navigator
  input: InputManager
  ui: UIController
  rootPlane: GraphPlane
  currentPlane: GraphPlane
  private frameId = 0
  private isVisible = true

  constructor(private canvasHost: HTMLElement, private uiLayer: HTMLElement) {
    this.stage = new Stage(canvasHost)
    this.navigator = new Navigator(this.stage.camera)
    this.input = new InputManager(this, this.stage.renderer.domElement)
    this.ui = new UIController(this, uiLayer)

    this.rootPlane = new GraphPlane('root', 0)
    this.currentPlane = this.rootPlane
    this.stage.scene.add(this.rootPlane.group)

    this.seedGraph()

    this.stage.camera.lookAt(0, 0, 0)
    this.animate()
  }

  private seedGraph() {
    this.rootPlane.addNode('node_0', 'Start')
    this.rootPlane.addNode('node_1', 'Process')
    this.rootPlane.addNode('node_2', 'Ship')
    this.rootPlane.addLink('node_0', 'node_1')
    this.rootPlane.addLink('node_1', 'node_2')

    const sub = this.rootPlane.nodes.get('node_0')!.ensureSubLayer()
    sub.addNode('sub_0', 'Child A')
    sub.addNode('sub_1', 'Child B')
    sub.addLink('sub_0', 'sub_1')

    const deeper = sub.nodes.get('sub_0')!.ensureSubLayer()
    deeper.addNode('deep_0', 'Leaf')

    LayoutEngine.apply(this.rootPlane)
  }

  resize() {
    this.stage.resize()
  }

  setVisible(visible: boolean) {
    this.isVisible = visible
  }

  dispose() {
    cancelAnimationFrame(this.frameId)
    this.input.dispose()
    this.stage.renderer.dispose()
    if (this.stage.renderer.domElement.parentElement) {
      this.stage.renderer.domElement.parentElement.removeChild(this.stage.renderer.domElement)
    }
  }

  private animate = () => {
    this.frameId = requestAnimationFrame(this.animate)
    if (!this.isVisible) return

    this.ui.updateBreadcrumbs()
    this.ui.updateMinimap()
    this.stage.render()
  }

  get hoveredNodeId(): string | null {
    let foundId: string | null = null
    const checkPlane = (plane: GraphPlane) => {
      plane.nodes.forEach(node => {
        const mat = node.mesh.material as THREE.MeshPhongMaterial
        if (mat.emissiveIntensity > 1.0) foundId = node.id
        if (node.subPlane) checkPlane(node.subPlane)
      })
    }
    checkPlane(this.rootPlane)
    return foundId
  }
}
