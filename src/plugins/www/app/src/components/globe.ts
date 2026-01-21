import * as d3 from 'd3';
import { feature } from 'topojson-client';

export function setupGlobe(container: HTMLElement) {
    // Clear container
    container.innerHTML = '';

    const width = container.clientWidth;
    const height = container.clientHeight;

    const svg = d3.select(container)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', `0 0 ${width} ${height}`)
        .style('opacity', '0.6')
        .style('position', 'absolute')
        .style('inset', '0');

    const projection = d3.geoOrthographic()
        .scale(Math.min(width, height) / 2.2)
        .center([0, 0])
        .rotate([0, -30])
        .translate([width / 2, height / 2]);

    const pathGenerator = d3.geoPath().projection(projection);

    // Globe outline
    const strokeColor = '#ffffff'; // Default to light mode or adjust based on theme. Let's use white for now as it's often on dark.

    svg.append('circle')
        .attr('fill', 'none')
        .attr('stroke', strokeColor)
        .attr('stroke-width', 0.5)
        .attr('stroke-opacity', 0.2)
        .attr('cx', width / 2)
        .attr('cy', height / 2)
        .attr('r', projection.scale());

    const map = svg.append('g');
    const dotsGroup = svg.append('g').attr('class', 'dots');

    // Graticule
    const graticule = d3.geoGraticule().step([20, 20]);
    map.append('path')
        .datum(graticule)
        .attr('class', 'graticule')
        .attr('d', pathGenerator as any)
        .attr('fill', 'none')
        .attr('stroke', strokeColor)
        .attr('stroke-width', 0.3)
        .attr('stroke-opacity', 0.15);

    // Random Dots Animation
    const createRandomDot = () => {
        const lat = Math.random() * 180 - 90;
        const lon = Math.random() * 360 - 180;
        const coords: [number, number] = [lon, lat];

        const projected = projection(coords);
        if (!projected) return;

        const size = 2 + Math.random() * 6;
        const baseDuration = 3000 + Math.random() * 3000;

        const dot = dotsGroup.append('circle')
            .datum(coords)
            .attr('cx', projected[0])
            .attr('cy', projected[1])
            .attr('r', 0)
            .attr('fill', '#1e40af') // Blue color
            .attr('opacity', 0);

        dot.transition()
            .duration(500)
            .attr('opacity', 0.8)
            .attr('r', size)
            .transition()
            .duration(baseDuration - 1000)
            .transition()
            .duration(500)
            .attr('opacity', 0)
            .attr('r', 0)
            .remove();
    };

    const dotIntervalId = setInterval(createRandomDot, 500);

    // Rotation Animation
    let rotation = 0;
    const timer = d3.timer(() => {
        rotation += 0.2;
        projection.rotate([rotation, -30]);

        // Update paths
        map.selectAll('path').attr('d', pathGenerator as any);

        // Update dots
        dotsGroup.selectAll('circle').each(function (d: any) {
            const projected = projection(d);
            if (projected) {
                // Check visibility
                const center = projection.rotate();
                const geoDistance = d3.geoDistance(d, [-center[0], -center[1]]);

                d3.select(this)
                    .attr('cx', projected[0])
                    .attr('cy', projected[1])
                    .attr('visibility', geoDistance > Math.PI / 2 ? 'hidden' : 'visible');
            }
        });

        // Update outline circle position if window resizes, but here we depend on resize handler
    });

    // Load World Data
    d3.json('https://cdn.jsdelivr.net/npm/world-atlas@2/land-110m.json').then((data: any) => {
        if (!data) return;
        // @ts-ignore - topojson types might need adjustment or ignore
        const land = feature(data, data.objects.land) as any;

        map.append('g')
            .attr('class', 'countries')
            .selectAll('path')
            .data(land.features)
            .enter()
            .append('path')
            .attr('d', (d: any) => pathGenerator(d))
            .attr('fill', 'none')
            .attr('stroke', strokeColor)
            .attr('stroke-width', 0.5)
            .attr('stroke-opacity', 0.4);
    });

    // Resize Handler
    const handleResize = () => {
        const newWidth = container.clientWidth;
        const newHeight = container.clientHeight;

        svg.attr('viewBox', `0 0 ${newWidth} ${newHeight}`);

        // Update projection center
        projection
            .scale(Math.min(newWidth, newHeight) / 2.2)
            .translate([newWidth / 2, newHeight / 2]);

        // Update outline
        svg.select('circle')
            .attr('cx', newWidth / 2)
            .attr('cy', newHeight / 2)
            .attr('r', projection.scale());

        // Everything else updates in timer loop or requires explicit update
        // Force immediate path update
        map.selectAll('path').attr('d', pathGenerator as any);
    };

    window.addEventListener('resize', handleResize);

    // Return cleanup function
    return () => {
        window.removeEventListener('resize', handleResize);
        clearInterval(dotIntervalId);
        timer.stop();
    };
}
