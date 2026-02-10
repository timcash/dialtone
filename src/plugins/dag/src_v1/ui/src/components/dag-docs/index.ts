import type { VisualizationControl } from '../../util/section'

export async function mountDagDocs(): Promise<VisualizationControl> {
  return {
    dispose: () => {},
    setVisible: (_v: boolean) => {}
  }
}
