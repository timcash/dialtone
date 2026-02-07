uniform vec3 uColor;
uniform float uTime;

void main() {
    // Calculate distance from center of point (0.0 to 0.5)
    vec2 coord = gl_PointCoord - vec2(0.5);
    float dist = length(coord);

    if (dist > 0.5) discard;

    // Soft glow falloff
    float strength = 1.0 - (dist * 2.0);
    strength = pow(strength, 1.5);

    // Pulse effect (very subtle)
    float pulse = 0.5 + 0.1 * sin(uTime * 1.5);
    
    // Final alpha (extremely low for additive blending)
    float alpha = strength * pulse * 0.05;

    gl_FragColor = vec4(uColor, alpha);
}
