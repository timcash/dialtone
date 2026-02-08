import { Menu } from "../util/menu";

type ThreeJsTemplateConfigOptions = {
    spinSpeed: number;
    onSpinChange: (value: number) => void;
};

export function setupThreeJsTemplateMenu(options: ThreeJsTemplateConfigOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

    menu.addHeader("Settings");

    menu.addSlider(
        "Spin",
        options.spinSpeed,
        0,
        1,
        0.01,
        options.onSpinChange,
        (v) => v.toFixed(2),
    );


}
