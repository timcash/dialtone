uniform vec3 uColor;
uniform float uOpacity;
uniform float uTime;

void main() {
    float glow = 0.5 + 0.5 * sin(uTime * 3.0);
    float alpha = uOpacity * (0.8 + 0.2 * glow);
    gl_FragColor = vec4(uColor, alpha);
}
