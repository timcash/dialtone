import { SectionManager } from './util/section';
import { HomeSection } from './components/home';
import { SettingsSection } from './components/settings';

const sections = new SectionManager();
(window as any).sections = sections;

sections.register('home', { component: HomeSection, header: { visible: true, menuVisible: true } });
sections.register('settings', { component: SettingsSection, header: { visible: false, menuVisible: false } });

(window as any).navigateTo = (id: string) => {
  sections.navigateTo(id);
};

window.addEventListener('hashchange', () => {
  const id = window.location.hash.slice(1) || 'home';
  sections.navigateTo(id);
});

// Initial load
const initialId = window.location.hash.slice(1) || 'home';
sections.navigateTo(initialId);
