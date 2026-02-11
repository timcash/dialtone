import { VisualizationControl, VisibilityMixin } from "@ui/ui";

export function mountTable(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();

    // If container is empty, inject default table layout
    if (!container.innerHTML.trim()) {
        container.innerHTML = `
            <div class="marketing-overlay" aria-label="Table Section">
                <h2>Process Table</h2>
                <p>High-density data visualization.</p>
            </div>
        `;
    }

    return {
        dispose: () => {
            container.innerHTML = '';
        },
        setVisible: (v) => VisibilityMixin.setVisible(state, v, 'table-viz'),
    };
}
