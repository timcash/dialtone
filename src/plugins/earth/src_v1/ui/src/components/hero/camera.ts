import * as THREE from 'three';

export class CameraManager {
  orbitCamera: THREE.PerspectiveCamera;
  topDownCamera: THREE.PerspectiveCamera;
  
  cameraDistance = 180;
  cameraOrbit = Math.PI;
  cameraOrbitSpeed = 0.1;
  cameraFarOffset = 80;
  cameraOrbitYOffset = -10;
  cameraShellOffset = 0.4;
  cameraTangentSpeed = 0.6;
  cameraYaw = 0.99;

  constructor(aspect: number) {
    this.orbitCamera = new THREE.PerspectiveCamera(75, aspect, 0.1, 10000);
    this.topDownCamera = new THREE.PerspectiveCamera(75, aspect, 0.1, 10000);
    
    // Static Top-Down Setup
    this.topDownCamera.position.set(0, 400, 0);
    this.topDownCamera.lookAt(0, 0, 0);
  }

  updateOrbit(ds: number, earthRadius: number) {
    this.cameraOrbit += this.cameraOrbitSpeed * ds;
    const near = earthRadius + Math.max(6, 23.5);
    const orbit = this.cameraOrbit + this.cameraYaw;
    
    this.orbitCamera.position.set(
      Math.cos(orbit) * (near + this.cameraFarOffset + (this.cameraDistance - 23.5)),
      this.cameraOrbitYOffset,
      Math.sin(orbit) * (near + (this.cameraDistance - 23.5) / 2),
    );
    
    this.orbitCamera.lookAt(
      new THREE.Vector3(
        Math.cos(orbit * this.cameraTangentSpeed) * (earthRadius + this.cameraShellOffset),
        0,
        Math.sin(orbit * this.cameraTangentSpeed) * (earthRadius + this.cameraShellOffset),
      ),
    );
  }

  setAspect(aspect: number) {
    this.orbitCamera.aspect = aspect;
    this.orbitCamera.updateProjectionMatrix();
    this.topDownCamera.aspect = aspect;
    this.topDownCamera.updateProjectionMatrix();
  }
}
