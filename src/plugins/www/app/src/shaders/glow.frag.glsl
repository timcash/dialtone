uniform vec3 uColor;
uniform float uIntensity;
uniform float uTime;

varying vec3 vNormal;
varying vec3 vPosition;

void main() {
    float pulse = 0.8 + 0.2 * sin(uTime * 2.0 + vPosition.x * 3.0 + vPosition.y * 2.0);
    float fresnel = pow(1.0 - abs(dot(vNormal, vec3(0.0, 0.0, 1.0))), 2.0);
    float glow = fresnel * uIntensity * pulse;
    vec3 color = uColor * (1.0 + glow * 0.5);
    float alpha = 0.6 + glow * 0.4;
    gl_FragColor = vec4(color, alpha);
}
