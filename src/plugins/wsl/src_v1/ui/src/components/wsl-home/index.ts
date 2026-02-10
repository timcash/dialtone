import { startTyping } from "../../util/typing";
import { mountWslViz } from "../wsl-viz";

export class HomeSection {
    private stopTyping: (() => void) | null = null;
    private viz: any = null;

    constructor(private container: HTMLElement) {}

    async mount() {
        const vizContainer = this.container.querySelector('.viz-container') as HTMLElement;
        if (vizContainer) {
            this.viz = mountWslViz(vizContainer);
        }

        const subtitleEl = this.container.querySelector('[data-typing-subtitle]') as HTMLElement;
        this.stopTyping = startTyping(subtitleEl, [
            "Alpine Linux nodes.",
            "Real-time telemetry.",
            "Windows host integration.",
        ]);
    }

    unmount() {
        if (this.stopTyping) this.stopTyping();
        if (this.viz) this.viz.dispose();
    }

    setVisible(visible: boolean) {
        if (this.viz) this.viz.setVisible(visible);
    }
}