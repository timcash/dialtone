import { VisualizationControl, VisibilityMixin } from "../../dialtone-ui";

export function mountSettings(_container: HTMLElement): VisualizationControl {
  const state = VisibilityMixin.defaults();

  return {
    dispose: () => {},
    setVisible: (v: boolean) =>
      VisibilityMixin.setVisible(state, v, "wsl-settings"),
  };
}
