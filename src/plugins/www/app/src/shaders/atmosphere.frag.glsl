varying vec3 vNormal;
uniform vec3 uSunDir;
uniform vec3 uSunColor;
uniform vec3 uKeyDir;
uniform vec3 uKeyColor;
uniform vec3 uKeyDir2;
uniform vec3 uKey2Color;
uniform float uKeyIntensity;
uniform float uKeyIntensity2;
uniform float uSunIntensity;
uniform float uAmbientIntensity;
uniform float uColorScale;
void main() {
    float fresnel = pow(0.7 - dot(vNormal, vec3(0, 0, 1.0)), 4.0);
    vec3 sunDir = normalize(uSunDir);
    vec3 keyDir = normalize(uKeyDir);
    vec3 keyDir2 = normalize(uKeyDir2);
    float diffuseSun = max(dot(vNormal, sunDir), 0.0);
    float diffuseKey = max(dot(vNormal, keyDir), 0.0);
    float diffuseKey2 = max(dot(vNormal, keyDir2), 0.0);
    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
    float boostedDiffuse = mix(diffuseKey, pow(diffuseKey, 0.65), ambientFactor);
    float boostedDiffuse2 = mix(diffuseKey2, pow(diffuseKey2, 0.65), ambientFactor);
    float sunTerm = pow(diffuseSun, 4.0) * uSunIntensity * 3.0;
    
    // Volumetric sun glow
    vec3 viewDir = normalize(vec3(0, 0, 1.0));
    float viewDotSun = max(dot(viewDir, sunDir), 0.0);
    float glow = pow(viewDotSun, 32.0) * uSunIntensity * 2.5;
    
    float key1 = boostedDiffuse * uKeyIntensity;
    float key2 = boostedDiffuse2 * uKeyIntensity2;
    vec3 light = vec3(uAmbientIntensity) + uSunColor * sunTerm + uKeyColor * key1 + uKey2Color * key2;
    vec3 color = vec4(0.3, 0.6, 1.0, 1.0).rgb * fresnel * light * uColorScale;
    vec3 finalGlow = uSunColor * glow * uColorScale;
    
    gl_FragColor = vec4(color + finalGlow, 1.0);
}
