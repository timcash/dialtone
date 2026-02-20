import { setupApp } from '../../../../../plugins/ui/src_v1/ui/ui';

try {
  const { sections } = setupApp({ title: 'simple-test', debug: true });

  sections.register('simple-three-stage', {
    containerId: 'simple-three-stage',
    load: async () => {
      // Minimal mock component
      return {
        setVisible: (visible: boolean) => {
          console.log('[SimpleTest] Section visibility:', visible);
        },
        dispose: () => {
          console.log('[SimpleTest] Disposing...');
        }
      };
    },
    header: { visible: true, title: 'Simple Stage' }
  });

  // Navigate to our section
  void sections.navigateTo('simple-three-stage');

  // Mark ready after a short simulation delay
  setTimeout(() => {
    const el = document.getElementById('simple-three-stage');
    if (el) el.setAttribute('data-ready', 'true');
    
    const header = document.querySelector('[aria-label="App Header"]');
    if (header) header.setAttribute('data-boot', 'true');
  }, 500);

} catch (err) {
  console.error('[SimpleTest] Setup failed:', err);
}
