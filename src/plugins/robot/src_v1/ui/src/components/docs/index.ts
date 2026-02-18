import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { marked } from 'marked';

export function mountDocs(container: HTMLElement): VisualizationControl {
  const content = container.querySelector('.docs-primary');
  
  const loadDocs = async () => {
    try {
      const res = await fetch('/docs/README.md');
      if (!res.ok) throw new Error(`Failed to load docs: ${res.status}`);
      const text = await res.text();
      if (content) {
        content.innerHTML = await marked.parse(text);
      }
    } catch (err) {
      if (content) {
        content.innerHTML = `<div class="error">Failed to load documentation.<br>Error: ${err}</div>`;
      }
    }
  };

  loadDocs();

  return {
    dispose: () => {},
    setVisible: (_visible: boolean) => {},
  };
}
