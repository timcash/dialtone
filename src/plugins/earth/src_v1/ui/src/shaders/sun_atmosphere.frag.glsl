varying vec3 vWorldPos;
varying vec3 vNormal;
uniform vec3 uSunDir;
uniform float uSunIntensity;
uniform float uAmbientIntensity;
uniform vec3 uCameraPos;
uniform float uColorScale;
void main() {
    vec3 normal = normalize(vNormal);
    vec3 viewDir = normalize(uCameraPos - vWorldPos);
    float rim = pow(1.0 - max(dot(normal, viewDir), 0.0), 3.0);
    float sunFacing = pow(max(dot(normal, normalize(uSunDir)), 0.0), 2.6);
    float sunBoost = (0.2 + uSunIntensity * 0.06);
    float ambientBoost = 0.15 + uAmbientIntensity * 0.2;
    float intensity = rim * (ambientBoost + sunFacing * sunBoost);
    vec3 color = vec3(0.35, 0.6, 1.0);
    gl_FragColor = vec4(color * intensity * uColorScale, intensity);
}
