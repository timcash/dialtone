uniform vec3 uColor;
uniform float uTime;

void main() {
    // Calculate distance from center of point (0.0 to 0.5)
    vec2 coord = gl_PointCoord - vec2(0.5);
    float dist = length(coord);

    if (dist > 0.5) discard;

    // Sharper glow falloff
    float strength = 1.0 - (dist * 2.0);
    strength = pow(strength, 3.0);

    // Pulse effect (very subtle)
    float pulse = 0.5 + 0.1 * sin(uTime * 1.5);
    
    // Final alpha (increased for better visibility)
    float alpha = strength * pulse * 0.15;

    gl_FragColor = vec4(uColor, alpha);
}
