import type { VisualizationControl } from '../../dialtone-ui'

export async function mountDagDocs(): Promise<VisualizationControl> {
  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  }
}
