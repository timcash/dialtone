varying vec3 vNormal;
uniform vec3 uSunDir;
uniform vec3 uKeyDir;
uniform float uKeyIntensity;
uniform float uSunIntensity;
uniform float uAmbientIntensity;
uniform float uColorScale;
void main() {
    float fresnel = pow(0.7 - dot(vNormal, vec3(0, 0, 1.0)), 4.0);
    vec3 sunDir = normalize(uSunDir);
    vec3 keyDir = normalize(uKeyDir);
    float diffuseSun = max(dot(vNormal, sunDir), 0.0);
    float diffuseKey = max(dot(vNormal, keyDir), 0.0);
    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
    float boostedDiffuse = mix(diffuseKey, pow(diffuseKey, 0.65), ambientFactor);
    float sunTerm = pow(diffuseSun, 4.0) * uSunIntensity * 3.0;
    
    // Volumetric sun glow
    vec3 viewDir = normalize(vec3(0, 0, 1.0));
    float viewDotSun = max(dot(viewDir, sunDir), 0.0);
    float glow = pow(viewDotSun, 32.0) * uSunIntensity * 2.5;
    
    float light = uAmbientIntensity + boostedDiffuse * uKeyIntensity + sunTerm;
    vec3 color = vec4(0.3, 0.6, 1.0, 1.0).rgb * fresnel * light * uColorScale;
    vec3 finalGlow = vec3(1.0, 0.9, 0.7) * glow * uColorScale;
    
    gl_FragColor = vec4(color + finalGlow, 1.0);
}
