import './../style.css'; // Import styles so Vite processes them
import { setupGlobe } from './components/globe';

// Initialize Globe
const globeContainer = document.getElementById('globe-container');
if (globeContainer) {
    setupGlobe(globeContainer);
}

// Video Lazy Loading
const videos = document.querySelectorAll('video');
const observer = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
        if (entry.isIntersecting) {
            const video = entry.target as HTMLVideoElement;
            // If it has source children, they might need loading logic, 
            // but for simple <video src="..."> or <source> with preload="none"
            // For now, assume standard autoplay behavior triggers when we ensure it's in view?
            // Actually, best practice for lazy load is data-src.
            // But let's stick to the React logic: it used src={isVisible ? src : undefined}.

            // In our HTML we put <source src="...">.
            // If we want lazy, we should have used data-src. 
            // The HTML I wrote has <source src="...">. 
            // The browser *might* preload metadata.

            // Let's implement play/pause based on visibility to save resources
            video.play().catch(e => console.log("Autoplay blocked", e));
        } else {
            const video = entry.target as HTMLVideoElement;
            video.pause();
        }
    });
}, { threshold: 0.1 });

videos.forEach(video => observer.observe(video));

console.log("Dialtone WWW Initialized");
