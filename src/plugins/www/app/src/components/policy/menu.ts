import { Menu } from "../util/menu";
import type { PolicyPreset } from "./types";

type PolicyMenuOptions = {
  presets: PolicyPreset[];
  activePresetId: string;
  orbitSpeed: number;
  volatility: number;
  monteCarloVisible: boolean;
  summaryText: string;
  onPresetChange: (presetId: string) => void;
  onOrbitSpeedChange: (value: number) => void;
  onVolatilityChange: (value: number) => void;
  onToggleMonteCarlo: () => void;
};

export function setupPolicyMenu(options: PolicyMenuOptions): void {
  const menu = Menu.getInstance();
  menu.clear();

  menu.addHeader("Markov Scenarios");
  for (const preset of options.presets) {
    menu.addButton(
      preset.label,
      () => options.onPresetChange(preset.id),
      preset.id === options.activePresetId,
    );
  }

  menu.addButton(
    options.monteCarloVisible ? "Hide Monte Carlo" : "Show Monte Carlo",
    options.onToggleMonteCarlo,
    false,
  );

  menu.addHeader("Model");
  menu.addSlider(
    "Orbit",
    options.orbitSpeed,
    0,
    0.55,
    0.01,
    options.onOrbitSpeedChange,
    (v) => v.toFixed(2),
  );

  menu.addSlider(
    "Volatility",
    options.volatility,
    0.1,
    2,
    0.05,
    options.onVolatilityChange,
    (v) => v.toFixed(2),
  );

  menu.addHeader("Run");
  const status = menu.addStatus();
  status.update(options.summaryText);
}
