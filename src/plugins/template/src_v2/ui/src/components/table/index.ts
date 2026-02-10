import { VisualizationControl } from "@ui/ui";

export function mountTable(container: HTMLElement): VisualizationControl {
    container.innerHTML = `
        <div class="marketing-overlay" aria-label="Table Section">
            <h2>Process Table</h2>
            <p>High-density data visualization.</p>
        </div>
    `;

    return {
        dispose: () => {
            container.innerHTML = '';
        },
        setVisible: (_v) => { },
    };
}
