uniform vec3 uColor;
uniform float uTime;
uniform float uGridSize;

varying vec2 vUv;
varying vec3 vPosition;

void main() {
    vec2 grid = abs(fract(vPosition.xz * uGridSize - 0.5) - 0.5) / fwidth(vPosition.xz * uGridSize);
    float line = min(grid.x, grid.y);
    float gridAlpha = 1.0 - min(line, 1.0);
    
    float pulse = 0.5 + 0.5 * sin(uTime * 1.5 + vPosition.x * 0.5 + vPosition.z * 0.3);
    float dist = length(vPosition.xz) * 0.1;
    float fade = exp(-dist * 0.3);
    
    vec3 color = uColor * (1.0 + pulse * 0.3);
    float alpha = gridAlpha * fade * (0.4 + pulse * 0.2);
    
    gl_FragColor = vec4(color, alpha);
}
