import { Menu } from "../../util/menu";

export function setupDemoMenu(viz: { spinSpeed: number }) {
    const menu = Menu.getInstance();
    menu.clear();
    menu.addHeader("Demo Settings");
    menu.addSlider("Spin Speed", viz.spinSpeed, 0, 2, 0.01, (v) => {
        viz.spinSpeed = v;
    }, (v) => v.toFixed(2));
}
