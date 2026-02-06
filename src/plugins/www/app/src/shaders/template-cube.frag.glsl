uniform vec3 uColor;
uniform vec3 uGlowColor;
uniform vec3 uLightDir;
uniform float uTime;

varying vec3 vNormal;
varying vec3 vViewPosition;

void main() {
  vec3 N = normalize(vNormal);
  vec3 V = normalize(vViewPosition);
  vec3 L = normalize(uLightDir);

  float diffuse = max(0.0, dot(N, L));
  float key = 0.4 + 0.6 * diffuse;

  float fresnel = pow(1.0 - max(0.0, dot(N, V)), 2.0);
  float glow = fresnel * (0.6 + 0.2 * sin(uTime));

  vec3 lit = uColor * key;
  vec3 rim = uGlowColor * glow;
  gl_FragColor = vec4(lit + rim, 1.0);
}
