import { VisualizationControl } from "../../util/ui";
import { startTyping } from "../../util/typing";

export class DocsVisualization {
    private stopTyping: (() => void) | null = null;

    constructor(private container: HTMLElement) { }

    async init() {
        const subtitleEl = this.container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
        this.stopTyping = startTyping(subtitleEl, [
            "Comprehensive documentation for your plugin.",
            "Explain features and CLI commands clearly.",
            "Guide users through installation and setup.",
        ]);
    }

    dispose() {
        if (this.stopTyping) this.stopTyping();
    }

    setVisible(_visible: boolean) { }
}

export function mountDocs(container: HTMLElement): VisualizationControl {
    const viz = new DocsVisualization(container);
    viz.init();
    return {
        dispose: () => viz.dispose(),
        setVisible: (v) => viz.setVisible(v)
    };
}