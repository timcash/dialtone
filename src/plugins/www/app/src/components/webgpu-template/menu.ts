import { Menu } from "../util/menu";

type WebGpuTemplateConfigOptions = {
    speed: number;
    onSpeedChange: (value: number) => void;
};

export function setupWebGpuTemplateMenu(options: WebGpuTemplateConfigOptions) {
    const menu = new Menu("webgpu-template-config-panel", "Menu");

    menu.addHeader("Settings");

    menu.addSlider(
        "Speed",
        options.speed,
        0,
        5,
        0.1,
        options.onSpeedChange,
        (v) => v.toFixed(1),
    );

    return {
        dispose: () => menu.dispose(),
        setToggleVisible: (visible: boolean) => menu.setToggleVisible(visible),
    };
}
