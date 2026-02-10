import './style.css'
import { SectionManager } from './util/section'
import { HomeSection } from './components/wsl-home'
import { SettingsSection } from './components/wsl-settings'
import { TableSection } from './components/wsl-table'

// 1. Core State
const sections = new SectionManager();
(window as any).sections = sections;

// 2. Register Sections
sections.register('wsl-home', { component: HomeSection, header: { visible: true } });
sections.register('wsl-settings', { component: SettingsSection, header: { visible: false } });
sections.register('wsl-table', { component: TableSection, header: { visible: false } });

// 3. Navigation Logic
(window as any).navigateTo = (id: string) => {
  sections.navigateTo(id);
};

window.addEventListener('hashchange', () => {
  const id = window.location.hash.slice(1) || 'wsl-home';
  sections.navigateTo(id);
});

// Initial load
const initialId = window.location.hash.slice(1) || 'wsl-home';
sections.navigateTo(initialId);

// 4. Global Menu Toggle
const menuToggle = document.getElementById('global-menu-toggle');
const menuPanel = document.getElementById('global-menu-panel');
if (menuToggle && menuPanel) {
    menuToggle.onclick = (e) => {
        e.stopPropagation();
        menuPanel.hidden = !menuPanel.hidden;
        menuToggle.setAttribute('aria-expanded', String(!menuPanel.hidden));
    };
}

// 5. Visibility Observer for Marketing Overlays (like www)
const marketingObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        entry.target.classList.toggle('is-visible', entry.isIntersecting);
    });
}, { threshold: 0.45 });

document.querySelectorAll('.snap-slide').forEach(slide => marketingObserver.observe(slide));
