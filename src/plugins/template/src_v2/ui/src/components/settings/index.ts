import { VisualizationControl } from "@ui/ui";

export function mountSettings(container: HTMLElement): VisualizationControl {
    container.innerHTML = `
        <div class="marketing-overlay" aria-label="Settings Section">
            <h2>Configuration</h2>
            <p>System settings and parameters.</p>
        </div>
    `;

    return {
        dispose: () => {
            container.innerHTML = '';
        },
        setVisible: (_v) => { },
    };
}