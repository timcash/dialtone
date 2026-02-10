import { setupApp } from './util/ui';
import { HeroSection } from './components/home';
import { DocsSection } from './components/docs';
import { TableSection } from './components/table';
import { SettingsSection } from './components/settings';

// 1. Initialize App with standard patterns
const { sections, menu } = setupApp({ title: 'dialtone.template' });

// 2. Register Sections
sections.register('home', { component: HeroSection, header: { visible: true } });
sections.register('docs', { component: DocsSection, header: { visible: true } });
sections.register('table', { component: TableSection, header: { visible: false } });
sections.register('settings', { component: SettingsSection, header: { visible: false } });

// 3. Setup Global Menu
menu.addHeader('Navigation');
menu.addButton('Hero Visualization', () => sections.navigateTo('home'));
menu.addButton('Documentation', () => sections.navigateTo('docs'));
menu.addButton('Spreadsheet', () => sections.navigateTo('table'));
menu.addButton('Configuration', () => sections.navigateTo('settings'));

// 4. Start Observation
sections.observe();

// 5. Initial load based on hash or default to home
const initialId = window.location.hash.slice(1) || 'home';
sections.mountAndShow(initialId);
