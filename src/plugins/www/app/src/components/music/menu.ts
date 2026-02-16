import { Menu } from "../util/menu";

type MusicConfigOptions = {
    sensitivity: number;
    onSensitivityChange: (value: number) => void;
    floor: number;
    onFloorChange: (value: number) => void;
    rotation: number;
    onRotationChange: (value: number) => void;
    enableMic: () => void;
    isMicEnabled: boolean;
    toggleDemo: () => void;
    isDemoMode: boolean;
    demoMode: string;
    toggleSound: () => void;
    isSoundOn: boolean;
};

export function setupMusicMenu(options: MusicConfigOptions): void {
    const menu = Menu.getInstance();
    menu.clear();

    menu.addHeader("Harmonic Analysis");

    const demoLabel = options.isDemoMode ? `Demo: ${options.demoMode}` : "Demo: OFF";
    menu.addButton(demoLabel, () => {
        options.toggleDemo();
    }, options.isDemoMode);

    menu.addButton(options.isSoundOn ? "Sound Off" : "Sound On", () => {
        options.toggleSound();
    }, options.isSoundOn);

    if (!options.isMicEnabled) {
        menu.addButton("Enable Microphone", () => {
            options.enableMic();
        }, true);
    } else {
        menu.addStatus().update("Microphone Active");
    }

    menu.addSlider(
        "Sensitivity",
        options.sensitivity,
        1,
        10,
        0.1,
        options.onSensitivityChange,
        (v) => v.toFixed(1),
    );

    menu.addSlider(
        "Floor",
        options.floor,
        -100,
        -20,
        1,
        options.onFloorChange,
        (v) => `${v} dB`,
    );

    menu.addSlider(
        "Rotation",
        options.rotation,
        0,
        Math.PI * 2,
        0.01,
        options.onRotationChange,
        (v) => `${((v / (Math.PI * 2)) * 360).toFixed(0)}°`,
    );
}
