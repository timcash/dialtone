import { Menu } from "../util/menu";

type CadConfigOptions = {
    params: {
        outer_diameter: number;
        inner_diameter: number;
        thickness: number;
        num_teeth: number;
        num_mounting_holes: number;
        mounting_hole_diameter: number;
    };
    translationX: number;
    onParamChange: (key: string, value: number) => void;
    onTranslationChange: (value: number) => void;
    onDownloadStl: () => void;
};

export function setupCadMenu(options: CadConfigOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

    menu.addHeader("Gear Parameters");

    const addParamSlider = (label: string, key: keyof typeof options.params, min: number, max: number, step: number) => {
        menu.addSlider(label, options.params[key], min, max, step, (v) => {
            options.onParamChange(key, v);
        });
    };

    addParamSlider("Outer Dia", "outer_diameter", 20, 200, 1);
    addParamSlider("Inner Dia", "inner_diameter", 5, 100, 1);
    addParamSlider("Thickness", "thickness", 2, 50, 1);
    addParamSlider("Num Teeth", "num_teeth", 5, 100, 1);
    addParamSlider("Mount Holes", "num_mounting_holes", 0, 12, 1);
    addParamSlider("Hole Dia", "mounting_hole_diameter", 2, 20, 1);

    menu.addHeader("Visualization");

    menu.addSlider("Translation X", options.translationX, -200, 200, 1, (v) => {
        options.onTranslationChange(v);
    });

    menu.addHeader("Actions");
    menu.addButton("Download STL", options.onDownloadStl, true);

    // Divider or GitHub link logic could be added here if Menu supported it or via custom element
    // For now, omitting the GitHub link as it's not core config, or I can add it as a button link?
    // I'll stick to core functionality.


}
