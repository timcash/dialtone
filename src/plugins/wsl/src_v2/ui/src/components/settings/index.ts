import { VisualizationControl, VisibilityMixin } from "@ui/ui";

export function mountSettings(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();

    return {
        dispose: () => {},
        setVisible: (v) => VisibilityMixin.setVisible(state, v, 'wsl-settings'),
    };
}
