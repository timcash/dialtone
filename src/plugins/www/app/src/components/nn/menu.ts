import { Menu } from "../util/menu";

type NnConfigOptions = {
    learningRate: number;
    batchSize: number;
    hiddenLayers: number;
    neuronsPerLayer: number;
    activation: string;
    optimizer: string;
    onConfigChange: (config: NnConfig) => void;
    onReset: () => void;
    onStep: () => void;
    togglePause: () => void;
    isPaused: boolean;
};

export type NnConfig = {
    learningRate: number;
    batchSize: number;
    hiddenLayers: number;
    neuronsPerLayer: number;
    activation: string;
    optimizer: string;
};

export function setupNnMenu(options: NnConfigOptions) {
    const menu = new Menu("nn-config-panel", "Menu");
    const config: NnConfig = {
        learningRate: options.learningRate,
        batchSize: options.batchSize,
        hiddenLayers: options.hiddenLayers,
        neuronsPerLayer: options.neuronsPerLayer,
        activation: options.activation,
        optimizer: options.optimizer,
    };

    const update = () => options.onConfigChange(config);

    // --- Network Architecture ---
    menu.addHeader("Architecture");

    menu.addSlider("Layers", config.hiddenLayers, 1, 10, 1, (v) => {
        config.hiddenLayers = v;
        update();
    });

    menu.addSlider("Neurons", config.neuronsPerLayer, 16, 256, 16, (v) => {
        config.neuronsPerLayer = v;
        update();
    });

    // --- Training ---
    menu.addHeader("Training");

    menu.addSlider("Learn Rate", config.learningRate, 0.0001, 0.1, 0.0001, (v) => {
        config.learningRate = v;
        update();
    }, (v) => v.toExponential(1));

    menu.addSlider("Batch Size", config.batchSize, 1, 128, 1, (v) => {
        config.batchSize = v;
        update();
    });

    // --- Controls ---
    menu.addHeader("Controls");

    const pauseBtn = menu.addButton(options.isPaused ? "Resume" : "Pause", () => {
        options.togglePause();
        pauseBtn.textContent = options.isPaused ? "Unknown" : "Unknown"; // Logic handled by caller usually? 
        // Wait, the caller toggles state. We might need to listen to state changes or just toggle button text locally if we trust it.
        // Let's toggle text based on assumption of success for now, or use a getter if we made one.
        // The shared Menu helper doesn't have a state binder for buttons yet.
        // Let's assume the button callback handles the logic.
        // Actually, let's just update text immediately
        pauseBtn.textContent = pauseBtn.textContent === "Pause" ? "Resume" : "Pause";
    }, true);

    menu.addButton("Step", options.onStep);

    menu.addButton("Reset Weights", options.onReset);

    return {
        dispose: () => menu.dispose(),
        setToggleVisible: (visible: boolean) => menu.setToggleVisible(visible),
    };
}
