import { VisualizationControl, VisibilityMixin } from "@ui/ui";
import { registerButtons, renderButtons } from "../../util/buttons";

export function mountSettings(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();

    registerButtons('settings', ['Default'], {
        'Default': [null, null, null, null, null, null, null, null]
    });

    return {
        dispose: () => {},
        setVisible: (v) => {
            if (v) renderButtons('settings');
            VisibilityMixin.setVisible(state, v, 'wsl-settings');
        },
    };
}
