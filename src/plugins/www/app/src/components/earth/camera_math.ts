import * as THREE from 'three';

export function applyHorizonConstraint(camera: THREE.PerspectiveCamera, earthRadius: number): void {
    const camPos = camera.position.clone();
    const dist = camPos.length();

    // Angle from cam-center vector to the horizon tangent point
    const hAngle = Math.asin(Math.min(0.99, earthRadius / dist));
    const toCenter = camPos.clone().negate().normalize();

    // Extract forward vector (0,0,-1) in camera local space
    const fwd = new THREE.Vector3(0, 0, -1).applyQuaternion(camera.quaternion);
    const ang = Math.acos(Math.min(1.0, fwd.dot(toCenter)));

    if (ang > hAngle * 0.94) {
        // The gaze is looking past the horizon. We must clamp it.
        const axis = new THREE.Vector3().crossVectors(toCenter, fwd).normalize();
        const clampedFwd = toCenter.clone().applyAxisAngle(axis, hAngle * 0.92);

        // We want to update the quaternion but maintain the 'up' orientation as best as possible.
        const m = new THREE.Matrix4().lookAt(camPos, camPos.clone().add(clampedFwd), camera.up);
        camera.quaternion.setFromRotationMatrix(m);
    }
}
