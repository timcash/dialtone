uniform float uSize;
uniform float uTime;
uniform float uPixelRatio;

void main() {
    vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
    gl_Position = projectionMatrix * mvPosition;
    
    // Size attenuation with DPI awareness (sharper base size)
    gl_PointSize = uSize * uPixelRatio * (80.0 / -mvPosition.z);
}
