import type { VisualizationControl } from '../../util/section'
import { DagApp } from '../../dag_viz/app'

export function mountDagLayerNest(container: HTMLElement): VisualizationControl {
  const section = container.closest('section')
  const uiLayer = section?.querySelector('#dag-layer-ui') as HTMLElement | null
  if (!uiLayer) {
    throw new Error('dag-layer-ui not found')
  }

  const app = new DagApp(container, uiLayer)
  const onResize = () => app.resize()
  window.addEventListener('resize', onResize)

  return {
    dispose: () => {
      window.removeEventListener('resize', onResize)
      app.dispose()
    },
    setVisible: (v: boolean) => app.setVisible(v)
  }
}
