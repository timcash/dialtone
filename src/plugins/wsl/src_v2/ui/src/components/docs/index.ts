import { VisualizationControl, VisibilityMixin, startTyping } from "@ui/ui";

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

    return {
        dispose: () => {
            stopTyping();
        },
        setVisible: (v) => VisibilityMixin.setVisible(state, v, 'wsl-docs'),
    };
}
