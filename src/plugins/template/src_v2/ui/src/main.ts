import './style.css'
import { SectionManager } from './util/section';
import { HomeSection } from './components/home';
import { TableSection } from './components/table';
import { SettingsSection } from './components/settings';

// 1. Core State
const sections = new SectionManager();
(window as any).sections = sections;

// 2. Register Sections
sections.register('home', { component: HomeSection, header: { visible: true, menuVisible: true } });
sections.register('table', { component: TableSection, header: { visible: false, menuVisible: false } });
sections.register('settings', { component: SettingsSection, header: { visible: false, menuVisible: false } });

// 3. Navigation Logic
(window as any).navigateTo = (id: string) => {
  sections.navigateTo(id);
};

window.addEventListener('hashchange', () => {
  const id = window.location.hash.slice(1) || 'home';
  sections.navigateTo(id);
});

// 4. Global Menu Toggle
const menuToggle = document.getElementById('global-menu-toggle');
const menuPanel = document.getElementById('global-menu-panel');

if (menuToggle && menuPanel) {
  menuToggle.addEventListener('click', () => {
    const isHidden = menuPanel.hasAttribute('hidden');
    if (isHidden) {
      menuPanel.removeAttribute('hidden');
      menuToggle.setAttribute('aria-expanded', 'true');
    } else {
      menuPanel.setAttribute('hidden', '');
      menuToggle.setAttribute('aria-expanded', 'false');
    }
  });

  // Close menu on click outside
  document.addEventListener('click', (e) => {
    if (!menuToggle.contains(e.target as Node) && !menuPanel.contains(e.target as Node)) {
      menuPanel.setAttribute('hidden', '');
      menuToggle.setAttribute('aria-expanded', 'false');
    }
  });
}

// Initial load
const initialId = window.location.hash.slice(1) || 'home';
sections.navigateTo(initialId);