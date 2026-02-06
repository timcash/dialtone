import './../style.css';
import { SectionManager } from './components/section';

// Create section manager for lazy loading Three.js components
const sections = new SectionManager({ debug: true });

// Register all Three.js sections
sections.register('s-home', {
    containerId: 'earth-container',
    load: async () => {
        const { mountEarth } = await import('./components/earth/index');
        const container = document.getElementById('earth-container');
        if (!container) throw new Error('earth-container not found');
        return mountEarth(container);
    }
});

sections.register('s-about', {
    containerId: 'about-container',
    header: { visible: false },
    load: async () => {
        const { mountAbout } = await import('./components/about/index');
        const container = document.getElementById('about-container');
        if (!container) throw new Error('about-container not found');
        return mountAbout(container);
    }
});

sections.register('s-robot', {
    containerId: 'robot-container',
    load: async () => {
        const { mountRobot } = await import('./components/robot/index');
        const container = document.getElementById('robot-container');
        if (!container) throw new Error('robot-container not found');
        return mountRobot(container);
    }
});

sections.register('s-neural', {
    containerId: 'nn-container',
    load: async () => {
        const { mountNeuralNetwork } = await import('./components/nn/index');
        const container = document.getElementById('nn-container');
        if (!container) throw new Error('nn-container not found');
        return mountNeuralNetwork(container);
    }
});

sections.register('s-math', {
    containerId: 'math-container',
    load: async () => {
        const { mountMath } = await import('./components/math/index');
        const container = document.getElementById('math-container');
        if (!container) throw new Error('math-container not found');
        return mountMath(container);
    }
});

sections.register('s-cad', {
    containerId: 'cad-container',
    load: async () => {
        const { mountCAD } = await import('./components/cad/index');
        const container = document.getElementById('cad-container');
        if (!container) throw new Error('cad-container not found');
        return mountCAD(container);
    }
});

sections.register('s-radio', {
    containerId: 'radio-container',
    load: async () => {
        const { mountRadio } = await import('./components/radio/index');
        const container = document.getElementById('radio-container');
        if (!container) throw new Error('radio-container not found');
        return mountRadio(container);
    }
});

sections.register('s-geotools', {
    containerId: 'geotools-container',
    header: { visible: false },
    load: async () => {
        const { mountGeoTools } = await import('./components/geotools/index');
        const container = document.getElementById('geotools-container');
        if (!container) throw new Error('geotools-container not found');
        return mountGeoTools(container);
    }
});

sections.register('s-docs', {
    containerId: 'docs-container',
    header: { visible: false },
    load: async () => {
        const { mountDocs } = await import('./components/docs/index');
        const container = document.getElementById('docs-container');
        if (!container) throw new Error('docs-container not found');
        return mountDocs(container);
    }
});

sections.register('s-webgpu-template', {
    containerId: 'webgpu-template-container',
    load: async () => {
        const { mountWebgpuTemplate } = await import('./components/webgpu-template/index');
        const container = document.getElementById('webgpu-template-container');
        if (!container) throw new Error('webgpu-template-container not found');
        return mountWebgpuTemplate(container);
    }
});

sections.register('s-threejs-template', {
    containerId: 'threejs-template-container',
    load: async () => {
        const { mountThreeJsTemplate } = await import('./components/threejs-template/index');
        const container = document.getElementById('threejs-template-container');
        if (!container) throw new Error('threejs-template-container not found');
        return mountThreeJsTemplate(container);
    }
});

// Start observing visibility and eagerly load first section
sections.observe();

// If URL has a section hash (e.g. from demo: #s-radio), scroll there and load that section first
const initialHash = window.location.hash.slice(1);
if (initialHash && document.getElementById(initialHash)?.classList.contains('snap-slide')) {
    sections.eagerLoad(initialHash);
    const section = document.getElementById(initialHash);
    if (section) {
        requestAnimationFrame(() => {
            section.scrollIntoView({ behavior: 'auto', block: 'start' });
        });
    }
} else {
    sections.eagerLoad('s-home');
}

const slides = document.querySelectorAll('.snap-slide[data-subtitle]');

// Update URL hash when scroll brings a section into view (so #s-threejs-template etc. stays in sync)
const allSlides = document.querySelectorAll('.snap-slide');
const hashObserver = new IntersectionObserver(
    (entries) => {
        let best: { id: string; ratio: number } | null = null;
        for (const entry of entries) {
            if (entry.isIntersecting && entry.intersectionRatio >= 0.5) {
                const id = (entry.target as HTMLElement).id;
                if (id && (!best || entry.intersectionRatio > best.ratio)) {
                    best = { id, ratio: entry.intersectionRatio };
                }
            }
        }
        if (best && location.hash.slice(1) !== best.id) {
            history.replaceState(null, '', '#' + best.id);
        }
    },
    { threshold: [0.5, 0.75, 1] }
);
allSlides.forEach((el) => hashObserver.observe(el));

// Marketing fade-in on section entry
const marketingObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        entry.target.classList.toggle('is-visible', entry.isIntersecting);
    });
}, { threshold: 0.45 });

slides.forEach(slide => marketingObserver.observe(slide));

// Video Lazy Loading
const videos = document.querySelectorAll('video');
const videoObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        const video = entry.target as HTMLVideoElement;
        if (entry.isIntersecting) {
            video.play().catch(e => console.log("Autoplay blocked", e));
        } else {
            video.pause();
        }
    });
}, { threshold: 0.1 });

videos.forEach(video => videoObserver.observe(video));
// Keyboard Navigation (Space bar to cycle slides)
window.addEventListener('keydown', (e) => {
    if (e.code === 'Space' || e.keyCode === 32) {
        e.preventDefault();
        const slides = Array.from(document.querySelectorAll('.snap-slide'));
        const currentSlideIndex = slides.findIndex(slide => {
            const rect = slide.getBoundingClientRect();
            return rect.top >= -10 && rect.top <= 10;
        });

        const nextIndex = (currentSlideIndex + 1) % slides.length;
        slides[nextIndex].scrollIntoView({ behavior: 'smooth' });
    }
});
