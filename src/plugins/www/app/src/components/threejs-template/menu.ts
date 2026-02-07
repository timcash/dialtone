import { Menu } from "../util/menu";

type ThreeJsTemplateConfigOptions = {
    spinSpeed: number;
    onSpinChange: (value: number) => void;
};

export function setupThreeJsTemplateMenu(options: ThreeJsTemplateConfigOptions) {
    const menu = new Menu("threejs-template-config-panel", "Menu");

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

    return {
        dispose: () => menu.dispose(),
        setToggleVisible: (visible: boolean) => menu.setToggleVisible(visible),
    };
}
