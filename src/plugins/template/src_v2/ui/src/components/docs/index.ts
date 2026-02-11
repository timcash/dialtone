import { VisualizationControl, VisibilityMixin, startTyping } from "../../dialtone-ui";

export function mountDocs(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();

    // If container is empty, inject default docs layout
    if (!container.innerHTML.trim()) {
        container.innerHTML = `
            <div class="marketing-overlay" aria-label="Docs Title">
                <h2>Documentation</h2>
                <p data-typing-subtitle></p>
            </div>
        `;
    }

    const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
    let stopTyping = () => {};
    
    if (subtitleEl) {
        stopTyping = startTyping(subtitleEl, [
            "Comprehensive documentation for your plugin.",
            "Explain features and CLI commands clearly.",
            "Guide users through installation and setup.",
        ]);
    }

    return {
        dispose: () => {
            stopTyping();
            container.innerHTML = '';
        },
        setVisible: (v) => VisibilityMixin.setVisible(state, v, 'docs-viz'),
    };
}
