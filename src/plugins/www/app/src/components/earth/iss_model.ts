import * as THREE from "three";

export function createISSModel(): THREE.Group {
  const issGroup = new THREE.Group();

  const body = new THREE.Mesh(
    new THREE.CylinderGeometry(0.02, 0.02, 0.15),
    new THREE.MeshStandardMaterial({
      color: 0xeeeeee,
      metalness: 0.95,
      roughness: 0.1,
    }),
  );
  body.rotation.z = Math.PI / 2;

  const panelGeo = new THREE.BoxGeometry(0.005, 0.08, 0.4);
  const panelMat = new THREE.MeshStandardMaterial({
    color: 0x888888,
    metalness: 0.9,
    roughness: 0.15,
  });

  const leftP = new THREE.Mesh(panelGeo, panelMat);
  const rightP = leftP.clone();
  leftP.position.x = -0.1;
  rightP.position.x = 0.1;

  issGroup.add(body, leftP, rightP);
  return issGroup;
}
