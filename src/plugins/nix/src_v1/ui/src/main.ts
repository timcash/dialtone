import './style.css'
import { SectionManager } from './util/section'
import { HeroSection } from './components/nix-hero'
import { DocsSection } from './components/nix-docs'
import { TableSection } from './components/nix-table'

// 1. Core State
const sections = new SectionManager();
(window as any).sections = sections;

// 2. Register Sections
sections.register('nix-hero', { component: HeroSection, header: { visible: true } });
sections.register('nix-docs', { component: DocsSection, header: { visible: false } });
sections.register('nix-table', { component: TableSection, header: { visible: false } });

// 3. Navigation Logic
(window as any).navigateTo = (id: string) => {
  sections.navigateTo(id);
};

window.addEventListener('hashchange', () => {
  const id = window.location.hash.slice(1) || 'nix-hero';
  sections.navigateTo(id);
});

// Initial load
const initialId = window.location.hash.slice(1) || 'nix-hero';
sections.navigateTo(initialId);