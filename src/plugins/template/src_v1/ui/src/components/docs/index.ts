import { SectionComponent } from "../../util/ui";
import { startTyping } from "../../util/typing";

export class DocsSection implements SectionComponent {
    private stopTyping: (() => void) | null = null;

    constructor(private container: HTMLElement) { }

    async mount() {
        const subtitleEl = this.container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement;
        this.stopTyping = startTyping(subtitleEl, [
            "Comprehensive documentation for your plugin.",
            "Explain features and CLI commands clearly.",
            "Guide users through installation and setup.",
        ]);
    }

    unmount() {
        if (this.stopTyping) this.stopTyping();
    }

    setVisible(_visible: boolean) { }
}
