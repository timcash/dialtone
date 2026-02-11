import { VisualizationControl, VisibilityMixin } from "@ui/ui";

export function mountSettings(container: HTMLElement): VisualizationControl {
    const state = VisibilityMixin.defaults();

    // If container is empty, inject default settings layout
    if (!container.innerHTML.trim()) {
        container.innerHTML = `
            <div class="marketing-overlay" aria-label="Settings Section">
                <h2>Configuration</h2>
                <p>Modify your plugin behavior here.</p>
            </div>
        `;
    }

    return {
        dispose: () => {
            container.innerHTML = '';
        },
        setVisible: (v) => VisibilityMixin.setVisible(state, v, 'settings-viz'),
    };
}
