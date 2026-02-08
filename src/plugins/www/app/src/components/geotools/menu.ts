import { Menu } from "../util/menu";

type GeoToolsConfigOptions = {
    currentResolution: number;
    onResolutionChange: (value: number) => void;
    onFile: (file: File) => Promise<void> | void;
    onConvert: () => void;
    onDownload: () => void;
    getStatusText: () => string;
    configCleanup?: () => void;
};

export function setupGeoToolsMenu(options: GeoToolsConfigOptions) {
    const menu = Menu.getInstance();
    menu.clear();

    // --- Section: Data ---
    menu.addHeader("Data Source");

    menu.addFile("Choose GeoJSON", async (file) => {
        await options.onFile(file);
        updateStatus();
    });

    // --- Section: View ---
    menu.addHeader("View Settings");

    menu.addSlider(
        "H3 Resolution",
        options.currentResolution,
        0,
        5,
        1,
        (v) => {
            options.onResolutionChange(v);
            updateStatus();
        },
        (v) => `Res ${v}`
    );

    // --- Section: Status ---
    const statusApi = menu.addStatus();

    // --- Section: Export ---
    menu.addHeader("Actions");

    menu.addButton("Convert to H3", () => {
        options.onConvert();
        updateStatus();
    }, true);

    menu.addButton("Download JSON", () => options.onDownload(), true);

    const updateStatus = () => {
        statusApi.update(options.getStatusText());
    };

    updateStatus();

    return {
        updateStatus,
    };
}
