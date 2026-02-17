import './../style.css';
import { SectionManager } from './components/util/section';
import { Menu } from './components/util/menu';

if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
        void navigator.serviceWorker.register('/sw.js').catch((err) => {
            console.error('[pwa] failed to register service worker', err);
        });
    });
}

// Create section manager for lazy loading Three.js components
const sections = new SectionManager();
(window as any).sections = sections;

// Register all Three.js sections
sections.register('s-home', {
    containerId: 'earth-container',
    load: async () => {
        const { mountEarth } = await import('./components/earth/index');
        const container = document.getElementById('earth-container');
        if (!container) throw new Error('earth-container not found');
        return mountEarth(container, sections);
    }
});

sections.register('s-about', {
    containerId: 'about-container',
    header: { visible: false },
    load: async () => {
        const { mountAbout } = await import('./components/about/index');
        const container = document.getElementById('about-container');
        if (!container) throw new Error('about-container not found');
        return mountAbout(container, sections);
    }
});

sections.register('s-robot', {
    containerId: 'robot-container',
    load: async () => {
        const { mountRobot } = await import('./components/robot/index');
        const container = document.getElementById('robot-container');
        if (!container) throw new Error('robot-container not found');
        return mountRobot(container, sections);
    }
});

sections.register('s-neural', {
    containerId: 'nn-container',
    load: async () => {
        const { mountNeuralNetwork } = await import('./components/nn/index');
        const container = document.getElementById('nn-container');
        if (!container) throw new Error('nn-container not found');
        return mountNeuralNetwork(container, sections);
    }
});

sections.register('s-math', {
    containerId: 'math-container',
    load: async () => {
        const { mountMath } = await import('./components/math/index');
        const container = document.getElementById('math-container');
        if (!container) throw new Error('math-container not found');
        return mountMath(container, sections);
    }
});

sections.register('s-cad', {
    containerId: 'cad-container',
    load: async () => {
        const { mountCAD } = await import('./components/cad/index');
        const container = document.getElementById('cad-container');
        if (!container) throw new Error('cad-container not found');
        return mountCAD(container, sections);
    }
});

sections.register('s-radio', {
    containerId: 'radio-container',
    load: async () => {
        const { mountRadio } = await import('./components/radio/index');
        const container = document.getElementById('radio-container');
        if (!container) throw new Error('radio-container not found');
        return mountRadio(container, sections);
    }
});

sections.register('s-geotools', {
    containerId: 'geotools-container',
    header: { visible: false },
    load: async () => {
        const { mountGeoTools } = await import('./components/geotools/index');
        const container = document.getElementById('geotools-container');
        if (!container) throw new Error('geotools-container not found');
        return mountGeoTools(container, sections);
    }
});

sections.register('s-policy', {
    containerId: 'policy-container',
    load: async () => {
        const { mountPolicy } = await import('./components/policy/index');
        const container = document.getElementById('policy-container');
        if (!container) throw new Error('policy-container not found');
        return mountPolicy(container, sections);
    }
});

sections.register('s-music', {
    containerId: 'music-container',
    load: async () => {
        const { mountMusic } = await import('./components/music/index');
        const container = document.getElementById('music-container');
        if (!container) throw new Error('music-container not found');
        return mountMusic(container, sections);
    }
});

sections.register('s-vision', {
    containerId: 'vision-container',
    load: async () => {
        const { mountVision } = await import('./components/vision/index');
        const container = document.getElementById('vision-container');
        if (!container) throw new Error('vision-container not found');
        return mountVision(container, sections);
    }
});

// Start observing visibility and eagerly load first section
sections.observe();

const loadSection = (id: string) => {
    const el = document.getElementById(id) as HTMLElement;
    if (id && el?.classList.contains('snap-slide')) {
        console.log(`[main] SWAP: #${id}`);
        
        // Pause and hide all other sections
        const allSlides = Array.from(document.querySelectorAll('.snap-slide')) as HTMLElement[];
        allSlides.forEach(slide => {
            if (slide.id !== id) {
                const control = sections.get(slide.id);
                if (control) control.setVisible(false);
                slide.style.display = 'none'; // Hide inactive slides
            }
        });

        // Show and snap active slide
        el.style.display = 'flex';
        sections.eagerLoad(id);
        sections.setActiveSection(id);
        
        console.log(`[main] EXECUTE SNAP to #${id}`);
        window.scrollTo(0, 0); // Always at top since only one slide is visible
        
        console.log(`[main] SETTLED: #${id}`);
        return true;
    }
    return false;
};

// Initial load
const initialHash = window.location.hash.slice(1) || 's-home';
loadSection(initialHash);

// Handle hash changes
window.addEventListener('hashchange', () => {
    const hash = window.location.hash.slice(1);
    if (hash) loadSection(hash);
});

// Navigation Logic
let lastNavTime = 0;
const NAV_COOLDOWN = 650;
let wheelGestureLocked = false;
let wheelGestureUnlockTimer: number | undefined;

const getActiveIndex = () => {
    const slides = Array.from(document.querySelectorAll('.snap-slide')) as HTMLElement[];
    return slides.findIndex(s => s.style.display !== 'none');
};

const navigate = (direction: 1 | -1) => {
    if (Date.now() - lastNavTime < NAV_COOLDOWN) return;
    const slides = Array.from(document.querySelectorAll('.snap-slide')) as HTMLElement[];
    const current = getActiveIndex();
    const next = Math.max(0, Math.min(slides.length - 1, current + direction));
    
    if (next !== current) {
        history.replaceState(null, '', '#' + slides[next].id);
        loadSection(slides[next].id);
        lastNavTime = Date.now();
    }
};

// Mouse Wheel
window.addEventListener('wheel', (e) => {
    if (Menu.getInstance().isOpen()) return;
    if (Math.abs(e.deltaY) < 30) return;
    if (wheelGestureLocked) {
        if (wheelGestureUnlockTimer) window.clearTimeout(wheelGestureUnlockTimer);
        wheelGestureUnlockTimer = window.setTimeout(() => {
            wheelGestureLocked = false;
        }, 180);
        return;
    }
    wheelGestureLocked = true;
    if (wheelGestureUnlockTimer) window.clearTimeout(wheelGestureUnlockTimer);
    wheelGestureUnlockTimer = window.setTimeout(() => {
        wheelGestureLocked = false;
    }, 180);
    navigate(e.deltaY > 0 ? 1 : -1);
}, { passive: true });

// Touch Handling
let touchStart = 0;
window.addEventListener('touchstart', (e) => {
    touchStart = e.touches[0].clientY;
}, { passive: true });

window.addEventListener('touchend', (e) => {
    if (Menu.getInstance().isOpen()) return;
    const touchEnd = e.changedTouches[0].clientY;
    const delta = touchStart - touchEnd;
    if (Math.abs(delta) > 50) {
        navigate(delta > 0 ? 1 : -1);
    }
}, { passive: true });

// Keyboard
window.addEventListener('keydown', (e) => {
    if (Menu.getInstance().isOpen()) return;
    if (e.code === 'Space' || e.key === 'ArrowDown') {
        e.preventDefault();
        navigate(1);
    } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        navigate(-1);
    }
});

// Marketing fade-in
const marketingObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        entry.target.classList.toggle('is-visible', entry.intersectionRatio >= 0.5);
    });
}, { threshold: [0.5] });
document.querySelectorAll('.snap-slide').forEach(slide => marketingObserver.observe(slide));

// Video Lazy Loading
const videoObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        const video = entry.target as HTMLVideoElement;
        if (entry.isIntersecting) {
            video.play().catch(() => {});
        } else {
            video.pause();
        }
    });
}, { threshold: 0.1 });
document.querySelectorAll('video').forEach(v => videoObserver.observe(v));
