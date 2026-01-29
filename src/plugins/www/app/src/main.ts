import './../style.css';
import { SectionManager } from './components/section';

// Create section manager for lazy loading Three.js components
const sections = new SectionManager({ debug: true });

// Register all Three.js sections
sections.register('s-home', {
    containerId: 'earth-container',
    load: async () => {
        const { mountEarth } = await import('./components/earth');
        const container = document.getElementById('earth-container');
        if (!container) throw new Error('earth-container not found');
        return mountEarth(container);
    }
});

sections.register('s-robot', {
    containerId: 'robot-container',
    load: async () => {
        const { mountRobot } = await import('./components/robot');
        const container = document.getElementById('robot-container');
        if (!container) throw new Error('robot-container not found');
        return mountRobot(container);
    }
});

sections.register('s-neural', {
    containerId: 'nn-container',
    load: async () => {
        const { mountNeuralNetwork } = await import('./components/nn');
        const container = document.getElementById('nn-container');
        if (!container) throw new Error('nn-container not found');
        return mountNeuralNetwork(container);
    }
});

sections.register('s-curriculum', {
    containerId: 'curriculum-container',
    load: async () => {
        const { mountBuildCurriculum } = await import('./components/build-curriculum');
        const container = document.getElementById('curriculum-container');
        if (!container) throw new Error('curriculum-container not found');
        return mountBuildCurriculum(container);
    }
});

// Start observing visibility and eagerly load first section
sections.observe();
sections.eagerLoad('s-home');

// Subtitle updates based on visible slide
const subtitleEl = document.getElementById('header-subtitle');
const slides = document.querySelectorAll('.snap-slide[data-subtitle]');

const subtitleObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting && subtitleEl) {
            const subtitle = (entry.target as HTMLElement).dataset.subtitle || '';
            subtitleEl.textContent = subtitle;
        }
    });
}, { threshold: 0.5 });

slides.forEach(slide => subtitleObserver.observe(slide));

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
