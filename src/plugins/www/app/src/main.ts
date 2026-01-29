import './../style.css';
import { mountEarth } from './components/earth';
import { mountNeuralNetwork } from './components/nn';
import { mountBuildCurriculum } from './components/build-curriculum';
import { mountRobot } from './components/robot';

// Initialize Earth
const earthContainer = document.getElementById('earth-container');
if (earthContainer) {
    mountEarth(earthContainer);
}

// Initialize Neural Network
const nnContainer = document.getElementById('nn-container');
if (nnContainer) {
    mountNeuralNetwork(nnContainer);
}

// Initialize Build Curriculum
const curriculumContainer = document.getElementById('curriculum-container');
if (curriculumContainer) {
    mountBuildCurriculum(curriculumContainer);
}

// Initialize Robot Arm
const robotContainer = document.getElementById('robot-container');
if (robotContainer) {
    mountRobot(robotContainer);
}

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

console.log("Dialtone WWW Initialized");
