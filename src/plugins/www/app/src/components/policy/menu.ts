import { Menu } from "../util/menu";

type PolicyMenuOptions = {
    domains: string[];
    orbitSpeed: number;
    onFundingChange: (index: number, value: number) => void;
    onOrbitSpeedChange: (value: number) => void;
};

export function setupPolicyMenu(options: PolicyMenuOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

    menu.addHeader("Camera");
    menu.addSlider(
        "Orbit",
        options.orbitSpeed,
        0,
        0.5,
        0.01,
        options.onOrbitSpeedChange,
        (v) => v.toFixed(2),
    );

    menu.addHeader("Policy Funding");
    for (let i = 0; i < options.domains.length; i++) {
        const idx = i;
        menu.addSlider(
            options.domains[i],
            50,
            0,
            100,
            1,
            (v) => options.onFundingChange(idx, v),
            (v) => Math.round(v).toString(),
        );
    }
}
