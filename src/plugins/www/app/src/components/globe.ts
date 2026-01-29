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
        .globeImageUrl('/earth-dark.jpg')
        .bumpImageUrl('/earth-topology.png')
        .showAtmosphere(true)
        .atmosphereColor('#3b82f6')
        .atmosphereDaylightAlpha(0.1);

    // Auto-rotation
    globe.controls().autoRotate = true;
    globe.controls().autoRotateSpeed = 0.5; // Serene rotation
    globe.controls().enableZoom = false; // Disable zoom for background globe

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
        globe.controls().autoRotate = false;
    };
}
