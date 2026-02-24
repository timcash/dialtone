import { VisualizationControl, VisibilityMixin, startTyping } from "@ui/ui";
import { registerButtons, renderButtons } from "../../util/buttons";

export function mountDocs(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();
    const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
    let stopTyping = () => {};
    
    if (subtitleEl) {
        stopTyping = startTyping(subtitleEl, [
            "Full control over your WSL nodes.",
            "Manage instance lifecycles via UI or CLI.",
            "Monitor resource usage in real-time.",
        ]);
    }

    registerButtons('docs', ['Default'], {
        'Default': [null, null, null, null, null, null, null, null]
    });

    return {
        dispose: () => {
            stopTyping();
        },
        setVisible: (v) => {
            if (v) renderButtons('docs');
            VisibilityMixin.setVisible(state, v, 'wsl-docs');
        },
    };
}
