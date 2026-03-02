import * as THREE from 'three';
import atmosphereVertexShader from '../../shaders/atmosphere.vert.glsl?raw';
import atmosphereFragmentShader from '../../shaders/atmosphere.frag.glsl?raw';
import sunAtmosphereVertexShader from '../../shaders/sun_atmosphere.vert.glsl?raw';
import sunAtmosphereFragmentShader from '../../shaders/sun_atmosphere.frag.glsl?raw';

const DEG_TO_RAD = Math.PI / 180;
const SUN_COLOR = new THREE.Color(1.0, 1.0, 1.0);
const KEY1_COLOR = new THREE.Color(0.9, 0.95, 1.0);
const KEY2_COLOR = new THREE.Color(0.85, 0.9, 1.0);
const MOON_LIGHT_LAYER = 1;

export class LightingManager {
  sunLight!: THREE.PointLight;
  sunGlow!: THREE.Mesh;
  atmosphereMaterial!: THREE.ShaderMaterial;
  sunAtmosphereMaterial!: THREE.ShaderMaterial;
  
  sunOrbitHeight = 870;
  sunOrbitSpeed = 0.0006283185307179586;
  sunOrbitIncline = 20 * DEG_TO_RAD;

  constructor(private scene: THREE.Scene, private earthRadius: number) {
    this.initLights();
    this.initAtmosphere();
  }

  private initLights() {
    const sunKeyLight = new THREE.DirectionalLight(0xffffff, 0.35);
    sunKeyLight.position.set(100, 50, 100);
    this.scene.add(sunKeyLight, sunKeyLight.target);

    this.scene.add(new THREE.AmbientLight(0x090a10, 0.26));
    this.scene.add(new THREE.HemisphereLight(0xffffff, 0x111111, 1.0));

    this.sunGlow = new THREE.Mesh(new THREE.SphereGeometry(60, 32, 32), new THREE.MeshBasicMaterial({ color: 0xffe08a }));
    this.scene.add(this.sunGlow);

    this.sunLight = new THREE.PointLight(0xffffff, 2.1, 220);
    this.sunLight.layers.enable(MOON_LIGHT_LAYER);
    this.scene.add(this.sunLight);
  }

  private initAtmosphere() {
    const geo = (r: number) => new THREE.SphereGeometry(r, 32, 32);

    this.atmosphereMaterial = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uSunColor: { value: SUN_COLOR.clone() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uKeyColor: { value: KEY1_COLOR.clone() },
        uKeyDir2: { value: new THREE.Vector3(0, 1, 0).normalize() },
        uKey2Color: { value: KEY2_COLOR.clone() },
        uKeyIntensity: { value: 0.8 },
        uKeyIntensity2: { value: 0.55 },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uColorScale: { value: 1.25 },
      },
      vertexShader: atmosphereVertexShader,
      fragmentShader: atmosphereFragmentShader,
    });
    this.scene.add(new THREE.Mesh(geo(this.earthRadius + 2.0), this.atmosphereMaterial));

    this.sunAtmosphereMaterial = new THREE.ShaderMaterial({
      side: THREE.BackSide,
      transparent: true,
      blending: THREE.AdditiveBlending,
      uniforms: {
        uSunDir: { value: new THREE.Vector3(1, 1, 1).normalize() },
        uKeyDir: { value: new THREE.Vector3(-1, 0, 0).normalize() },
        uSunIntensity: { value: 0.5 },
        uAmbientIntensity: { value: 0.1 },
        uCameraPos: { value: new THREE.Vector3() },
        uColorScale: { value: 1.25 },
      },
      vertexShader: sunAtmosphereVertexShader,
      fragmentShader: sunAtmosphereFragmentShader,
    });
    this.scene.add(new THREE.Mesh(geo(this.earthRadius + 3.2), this.sunAtmosphereMaterial));
  }

  update(now: number, cameraPos: THREE.Vector3) {
    const sunRad = this.earthRadius + this.sunOrbitHeight;
    const sunA = now * this.sunOrbitSpeed;
    const ky = Math.sin(sunA) * Math.sin(this.sunOrbitIncline) * sunRad;
    const kz = Math.sin(sunA) * Math.cos(this.sunOrbitIncline) * sunRad;
    this.sunLight.position.set(Math.cos(sunA) * sunRad, ky, kz);
    this.sunGlow.position.copy(this.sunLight.position);

    const sDir = this.sunLight.position.clone().normalize();
    this.atmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uSunDir.value.copy(sDir);
    this.sunAtmosphereMaterial.uniforms.uCameraPos.value.copy(cameraPos);
    
    return { sunDir: sDir, sunColor: SUN_COLOR };
  }
}
