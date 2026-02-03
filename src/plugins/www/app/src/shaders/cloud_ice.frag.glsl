uniform float uOpacity;
uniform float uTime;
uniform vec3 uSunDir;
uniform vec3 uSunColor;
uniform vec3 uKeyDir;
uniform vec3 uKeyColor;
uniform vec3 uKeyDir2;
uniform vec3 uKey2Color;
uniform float uSunIntensity;
uniform float uKeyIntensity;
uniform float uKeyIntensity2;
uniform float uAmbientIntensity;
uniform vec3 uTint;
uniform float uColorScale;
uniform float uGlow;
uniform float uCloudAmount;
varying vec3 vPosition;
varying vec3 vNormal;

vec3 mod289(vec3 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 mod289(vec4 x) { return x - floor(x * (1.0 / 289.0)) * 289.0; }
vec4 permute(vec4 x) { return mod289(((x*34.0)+1.0)*x); }
vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }
float snoise(vec3 v) {
  const vec2  C = vec2(1.0/6.0, 1.0/3.0) ;
  const vec4  D = vec4(0.0, 0.5, 1.0, 2.0);
  vec3 i  = floor(v + dot(v, C.yyy) );
  vec3 x0 = v - i + dot(i, C.xxx) ;
  vec3 g = step(x0.yzx, x0.xyz);
  vec3 l = 1.0 - g;
  vec3 i1 = min( g.xyz, l.zxy );
  vec3 i2 = max( g.xyz, l.zxy );
  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;
  i = mod289(i);
  vec4 p = permute( permute( permute(
              i.z + vec4(0.0, i1.z, i2.z, 1.0 ))
            + i.y + vec4(0.0, i1.y, i2.y, 1.0 ))
            + i.x + vec4(0.0, i1.x, i2.x, 1.0 ));
  float n_ = 0.142857142857;
  vec3  ns = n_ * D.wyz - D.xzx;
  vec4 j = p - 49.0 * floor(p * ns.z * ns.z);
  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_ );
  vec4 x = x_ *ns.x + ns.yyyy;
  vec4 y = y_ *ns.x + ns.yyyy;
  vec4 h = 1.0 - abs(x) - abs(y);
  vec4 b0 = vec4( x.xy, y.xy );
  vec4 b1 = vec4( x.zw, y.zw );
  vec4 s0 = floor(b0)*2.0 + 1.0;
  vec4 s1 = floor(b1)*2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));
  vec4 a0 = b0.xzyw + s0.xzyw*sh.xxyy ;
  vec4 a1 = b1.xzyw + s1.xzyw*sh.zzww ;
  vec3 p0 = vec3(a0.xy,h.x);
  vec3 p1 = vec3(a0.zw,h.y);
  vec3 p2 = vec3(a1.xy,h.z);
  vec3 p3 = vec3(a1.zw,h.w);
  vec4 norm = taylorInvSqrt(vec4(dot(p0,p0), dot(p1,p1), dot(p2, p2), dot(p3,p3)));
  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
  vec4 m = max(0.6 - vec4(dot(x0,x0), dot(x1,x1), dot(x2,x2), dot(x3,x3)), 0.0);
  m = m * m;
  return 42.0 * dot( m*m, vec4( dot(p0,x0), dot(p1,x1), dot(p2,x2), dot(p3,x3) ) );
}
float fbm(vec3 p) {
  float v = 0.0;
  float a = 0.5;
  vec3 shift = vec3(100.0);
  for (int i = 0; i < 4; ++i) {
    v += a * snoise(p);
    p = p * 2.0 + shift;
    a *= 0.5;
  }
  return v;
}

void main() {
    // Domain Warping
    vec3 warpShift = vec3(uTime * 0.03, uTime * 0.02, uTime * 0.04);
    vec3 q = vec3(
        fbm(vPosition * (CLOUD_SCALE * 0.35) + warpShift),
        fbm(vPosition * (CLOUD_SCALE * 0.35) + warpShift + vec3(1.2, 4.3, 7.1)),
        fbm(vPosition * (CLOUD_SCALE * 0.35) + warpShift + vec3(8.7, 2.2, 1.8))
    );

    // Multi-octave FBM for base density
    float nBase = fbm(vPosition * (CLOUD_SCALE * 0.7) + q * 1.3 + uTime * 0.06);
    
    // Atmospheric "Breathing" Oscillation
    float baseThreshold = mix(0.5, -0.1, uCloudAmount);
    float breath = sin(uTime * 0.15) * 0.05;
    float threshold = baseThreshold + breath;
    float alpha = smoothstep(threshold, threshold + 0.4, nBase) * uOpacity;

    vec3 sunDir = normalize(uSunDir);
    vec3 keyDir = normalize(uKeyDir);
    vec3 keyDir2 = normalize(uKeyDir2);
    float diffuseSun = max(dot(vNormal, sunDir), 0.0);
    float diffuseKey = max(dot(vNormal, keyDir), 0.0);
    float diffuseKey2 = max(dot(vNormal, keyDir2), 0.0);
    float ambientFactor = clamp(1.0 - uAmbientIntensity, 0.0, 1.0);
    float boostedDiffuse = mix(diffuseKey, pow(diffuseKey, 0.65), ambientFactor);
    float boostedDiffuse2 = mix(diffuseKey2, pow(diffuseKey2, 0.65), ambientFactor);
    float sunTerm = pow(diffuseSun, 2.6) * uSunIntensity * 0.12;
    float key1 = boostedDiffuse * uKeyIntensity;
    float key2 = boostedDiffuse2 * uKeyIntensity2;
    vec3 light = vec3(uAmbientIntensity) + uSunColor * sunTerm + uKeyColor * key1 + uKey2Color * key2;
    
    float rim = pow(1.0 - max(dot(vNormal, keyDir), 0.0), 3.0);
    vec3 glow = uTint * (rim * uGlow);
    vec3 litColor = uTint * light * uColorScale + glow;
    gl_FragColor = vec4(litColor, alpha);
}
