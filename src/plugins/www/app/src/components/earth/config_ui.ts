import { ProceduralOrbit } from '../earth';

export function setupConfigPanel(orbit: ProceduralOrbit) {
    const panel = document.getElementById('earth-config-panel') as HTMLDivElement | null;
    const toggle = document.getElementById('earth-config-toggle') as HTMLButtonElement | null;
    if (!panel || !toggle) return { panel: null, toggle: null };

    const setOpen = (open: boolean) => {
        panel.hidden = !open;
        panel.style.display = open ? 'grid' : 'none';
        toggle.setAttribute('aria-expanded', String(open));
    };

    setOpen(false);
    toggle.addEventListener('click', (e) => {
        e.preventDefault();
        e.stopPropagation();
        setOpen(panel.hidden);
    });

    const addSection = (title: string) => {
        const header = document.createElement('h3');
        header.textContent = title;
        panel.appendChild(header);
    };

    const addSlider = (key: string, label: string, value: number, min: number, max: number, step: number, onInput: (v: number) => void, format: (v: number) => string = (v) => v.toFixed(3)) => {
        const row = document.createElement('div');
        row.className = 'earth-config-row';
        const labelWrap = document.createElement('label');
        labelWrap.textContent = label;
        const slider = document.createElement('input');
        slider.type = 'range'; slider.min = `${min}`; slider.max = `${max}`; slider.step = `${step}`; slider.value = `${value}`;
        labelWrap.appendChild(slider);
        row.appendChild(labelWrap);
        const valueEl = document.createElement('span');
        valueEl.className = 'earth-config-value';
        valueEl.textContent = format(value);
        row.appendChild(valueEl);
        panel.appendChild(row);
        orbit.configValueMap.set(key, valueEl);
        slider.addEventListener('input', () => {
            const next = parseFloat(slider.value);
            onInput(next);
            valueEl.textContent = format(next);
        });
    };

    const addCopyButton = () => {
        const btn = document.createElement('button');
        btn.textContent = 'Copy Config';
        btn.addEventListener('click', () => {
            const payload = JSON.stringify(orbit.buildConfigSnapshot(), null, 2);
            navigator.clipboard?.writeText(payload);
        });
        panel.appendChild(btn);
    };

    addSection('Rotation');
    addSlider('earthRot', 'Earth Rot', orbit.earthRotSpeed, 0, 0.0002, 0.000001, (v: number) => orbit.earthRotSpeed = v, (v: number) => v.toFixed(6));
    addSlider('sunOrbitSpeed', 'Sun Orbit', orbit.sunOrbitSpeed, 0, 0.005, 0.0001, (v: number) => orbit.sunOrbitSpeed = v, (v: number) => v.toFixed(4));

    addSection('Camera');
    addSlider('distance', 'Distance', orbit.cameraDistance, 8, 30, 0.5, (v: number) => orbit.cameraDistance = v, (v: number) => v.toFixed(1));

    addCopyButton();

    return {
        panel,
        toggle,
        setOpen
    };
}

export function updateTelemetry(orbit: ProceduralOrbit, orbitRadius: number) {
    const kmPerUnit = 6371 / orbit.earthRadius;
    const altitudeKm = (orbitRadius - orbit.earthRadius) * kmPerUnit;
    if (orbit.altitudeEl) orbit.altitudeEl.textContent = `${altitudeKm.toFixed(0)} KM`;
}
