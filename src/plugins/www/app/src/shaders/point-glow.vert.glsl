uniform float uSize;
uniform float uTime;

void main() {
    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
    gl_Position = projectionMatrix * mvPosition;
    
    // Size attenuation
    gl_PointSize = uSize * (300.0 / -mvPosition.z);
}
