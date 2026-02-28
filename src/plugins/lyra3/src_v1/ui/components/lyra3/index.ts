import { VisualizationControl } from '../../../../ui/ui/types';

export function mountLyra3(container: HTMLElement): VisualizationControl {
  container.innerHTML = `
    <div class="lyra3-ui">
      <h1>Lyra3 Music Generation</h1>
      <p>Use the Google Lyria AI music model to create tracks.</p>
      <div class="prompt-form">
        <textarea id="lyra3-prompt" placeholder="Enter your music prompt (e.g., 'early 90s hip-hop with a fast tempo')"></textarea>
        <button id="lyra3-generate">Generate</button>
      </div>
      <div class="track-list" id="lyra3-tracks">
        <p>No tracks generated yet.</p>
      </div>
    </div>
  `;

  const generateBtn = container.querySelector('#lyra3-generate');
  generateBtn?.addEventListener('click', () => {
    const prompt = (container.querySelector('#lyra3-prompt') as HTMLTextAreaElement).value;
    console.log(`[Lyra3] Requesting generation for: ${prompt}`);
    // Handle generation logic here
  });

  return {
    dispose: () => {
      // Cleanup logic here
    },
    setVisible: (visible: boolean) => {
      if (visible) {
        container.classList.remove('hidden');
      } else {
        container.classList.add('hidden');
      }
    }
  };
}
