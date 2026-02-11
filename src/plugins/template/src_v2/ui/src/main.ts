import { setupApp } from './dialtone-ui';

// 1. Initialize App with standard patterns
const { sections, menu } = setupApp({ 
    title: 'dialtone.template',
    debug: true
});

// 2. Register Sections with Lazy Loading
sections.register('home', { 
    containerId: 'home',
    load: async () => {
        const { mountHero } = await import('./components/home/index');
        const container = document.getElementById('home');
        if (!container) throw new Error('home container not found');
        return mountHero(container);
    },
    header: { visible: true } 
});

sections.register('docs', { 
    containerId: 'docs',
    load: async () => {
        const { mountDocs } = await import('./components/docs/index');
        const container = document.getElementById('docs');
        if (!container) throw new Error('docs container not found');
        return mountDocs(container);
    },
    header: { visible: true, menuVisible: false } 
});

sections.register('table', { 
    containerId: 'table',
    load: async () => {
        const { mountTable } = await import('./components/table/index');
        const container = document.getElementById('table');
        if (!container) throw new Error('table container not found');
        return mountTable(container);
    },
    header: { visible: false, menuVisible: false } 
});

sections.register('settings', { 
    containerId: 'settings',
    load: async () => {
        const { mountSettings } = await import('./components/settings/index');
        const container = document.getElementById('settings');
        if (!container) throw new Error('settings container not found');
        return mountSettings(container);
    },
    header: { visible: false } 
});

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
console.log(`[SectionManager] 🧭 INITIAL LOAD #${initialId}`);
sections.load(initialId).then(() => {
    const el = document.getElementById(initialId);
    if (el) el.scrollIntoView({ behavior: 'auto' });
});
