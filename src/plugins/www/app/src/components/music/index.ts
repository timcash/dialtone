import { startTyping } from "../util/typing";
import { setupMusicMenu } from "./menu";
import { MusicVisualization } from "./visualization";

export function mountMusic(container: HTMLElement) {
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="Music section: frequency circle">
      <h2>Harmonic Vision</h2>
      <p data-typing-subtitle></p>
    </div>
    <div class="music-legend">
      <div class="legend-item"><span class="legend-line" style="background: #ff00ff"></span> Major 3rd</div>
      <div class="legend-item"><span class="legend-line" style="background: #00ffff"></span> Minor 3rd</div>
      <div class="legend-item"><span class="legend-line" style="background: #ffff00"></span> Perfect 5th</div>
    </div>
    <style>
      .music-legend {
        position: absolute;
        bottom: 2rem;
        left: 2rem;
        z-index: 20;
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        background: rgba(0, 0, 0, 0.4);
        padding: 1rem;
        border-radius: 8px;
        backdrop-filter: blur(4px);
        border: 1px solid rgba(255, 255, 255, 0.1);
        pointer-events: none;
      }
      .legend-item {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        color: rgba(255, 255, 255, 0.6);
        font-size: 0.85rem;
        font-family: var(--font-family);
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }
      .legend-line {
        width: 24px;
        height: 2px;
        display: inline-block;
        box-shadow: 0 0 8px currentColor;
      }
    </style>
  `;

  const subtitleEl = container.querySelector("[data-typing-subtitle]") as HTMLParagraphElement | null;
  const subtitles = [
    "Visualize sound through the Circle of Fifths.",
    "Live chromagram analysis from your microphone.",
    "Mapping frequencies to musical geometry.",
  ];
  const stopTyping = startTyping(subtitleEl, subtitles);

  const viz = new MusicVisualization(container);
  
  const updateMenu = () => {
    setupMusicMenu({
      sensitivity: viz.sensitivity,
      onSensitivityChange: (v) => { viz.sensitivity = v; },
      floor: viz.floor,
      onFloorChange: (v) => { viz.floor = v; },
      rotation: viz.rotation,
      onRotationChange: (v) => { viz.rotation = v; },
      enableMic: async () => {
        await viz.enableMic();
        updateMenu();
      },
      isMicEnabled: viz.analyzer.isActive,
      toggleDemo: async () => {
        await viz.toggleDemo();
        updateMenu();
      },
      isDemoMode: viz.isDemoMode,
      demoMode: viz.demoMode,
      toggleSound: async () => {
          await viz.toggleSound();
          updateMenu();
      },
      isSoundOn: viz.analyzer.isSoundOn
    });
  };

  // Initial menu setup
  updateMenu();

  return {
    dispose: () => {
      viz.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viz.setVisible(visible);
    },
    updateUI: () => {
      updateMenu();
    }
  };
}
