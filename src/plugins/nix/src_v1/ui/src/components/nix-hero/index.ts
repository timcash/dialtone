import { mountNixViz } from '../nix-viz';

export class HeroSection {
    private viz: any;

    constructor(private container: HTMLElement) {}

    async mount() {
        const vizContainer = this.container.querySelector('#viz-container') as HTMLElement;
        if (vizContainer) {
            this.viz = mountNixViz(vizContainer);
        }
    }

    unmount() {
        if (this.viz) {
            this.viz.dispose();
            this.viz = null;
        }
    }

    setVisible(visible: boolean) {
        if (this.viz) {
            this.viz.setVisible(visible);
        }
    }
}
