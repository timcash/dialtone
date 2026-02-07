import { Menu } from "../util/menu";

type WebGpuTemplateConfigOptions = {
    speed: number;
    onSpeedChange: (value: number) => void;
};

export function setupWebGpuTemplateMenu(options: WebGpuTemplateConfigOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

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


}
