import Globe from 'globe.gl';

export function setupGlobe(container: HTMLElement) {
    // Clear container
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight;

    // Initialize Globe
    const globe = Globe()(container)
        .width(width)
        .height(height)
        .backgroundColor('rgba(0,0,0,0)') // Transparent background
        .showAtmosphere(true)
        .atmosphereColor('#3b82f6')
        .atmosphereDaylightAlpha(0.1)
        .globeImageUrl('//unpkg.com/three-globe/example/img/earth-night.jpg') // Night view for premium feel
        .bumpImageUrl('//unpkg.com/three-globe/example/img/earth-topology.png')
        .pointColor(() => '#3b82f6') // Blue dots
        .pointAltitude(0.01)
        .pointRadius(0.5);

    // Auto-rotation
    globe.controls().autoRotate = true;
    globe.controls().autoRotateSpeed = 0.5;
    globe.controls().enableZoom = false; // Disable zoom for background globe

    // Random Dots (Points) Data
    const generateDots = () => {
        const dots = [];
        for (let i = 0; i < 20; i++) {
            dots.push({
                lat: Math.random() * 180 - 90,
                lng: Math.random() * 360 - 180,
                size: Math.random() * 0.5 + 0.2
            });
        }
        return dots;
    };

    globe.pointsData(generateDots());

    // Periodically update dots to mimic animation
    const dotIntervalId = setInterval(() => {
        globe.pointsData(generateDots());
    }, 4000);

    // Resize Handler
    const handleResize = () => {
        const newWidth = container.clientWidth;
        const newHeight = container.clientHeight;
        globe.width(newWidth).height(newHeight);
    };

    window.addEventListener('resize', handleResize);

    // Return cleanup function
    return () => {
        window.removeEventListener('resize', handleResize);
        clearInterval(dotIntervalId);
        // globe.gl doesn't have a formal destroy() but we can stop the loop if needed
        // Most Three.js based libs clean up when the container is removed or we can stop animations
        globe.controls().autoRotate = false;
    };
}
