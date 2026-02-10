import { VisualizationControl, startTyping } from "@ui/ui";

export function mountDocs(container: HTMLElement): VisualizationControl {
    container.innerHTML = `
        <div class="marketing-overlay" aria-label="Docs Title">
            <h2>Documentation</h2>
            <p data-typing-subtitle></p>
        </div>
    `;

    const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
    const stopTyping = startTyping(subtitleEl, [
        "Comprehensive documentation for your plugin.",
        "Explain features and CLI commands clearly.",
        "Guide users through installation and setup.",
    ]);

    return {
        dispose: () => {
            stopTyping();
            container.innerHTML = '';
        },
        setVisible: (_v) => { },
    };
}