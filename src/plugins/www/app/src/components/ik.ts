import * as THREE from "three";

// ============================================================================
// IK Joint Interface - What the IK solver needs from each joint
// ============================================================================
export interface IKJoint {
  mesh: THREE.Object3D;
  group: THREE.Object3D;
  axis: "x" | "y" | "z";
  getAngleRadians(): number;
  setAngleRadians(radians: number): void;
}

// ============================================================================
// IK Chain Interface - What the IK solver needs from the robot arm
// ============================================================================
export interface IKChain {
  base: THREE.Object3D;
  joints: IKJoint[];
  getEndEffectorPosition(): THREE.Vector3;
}

// ============================================================================
// Inverse Kinematics Solver (CCD - Cyclic Coordinate Descent)
// ============================================================================
export class IKSolver {
  chain: IKChain;
  damping: number = 0.05;
  maxIterations: number = 5;

  constructor(chain: IKChain, options?: { damping?: number }) {
    this.chain = chain;
    if (options?.damping !== undefined) {
      this.damping = options.damping;
    }
  }

  /**
   * Run one step of IK (call each frame for smooth motion)
   * @returns Current distance from end effector to target
   */
  step(target: THREE.Vector3): number {
    // Update matrices
    this.chain.base.updateMatrixWorld(true);

    // Iterate through joints from end to base (CCD order)
    for (let i = this.chain.joints.length - 1; i >= 0; i--) {
      this.adjustJoint(i, target);
    }

    // Return current distance to target
    const endPos = this.chain.getEndEffectorPosition();
    return endPos.distanceTo(target);
  }

  /**
   * Run multiple iterations of IK
   * @returns Final distance from end effector to target
   */
  solve(target: THREE.Vector3, iterations?: number): number {
    const iters = iterations ?? this.maxIterations;
    let distance = Infinity;

    for (let i = 0; i < iters; i++) {
      distance = this.step(target);
    }

    return distance;
  }

  private adjustJoint(jointIndex: number, target: THREE.Vector3): void {
    const joint = this.chain.joints[jointIndex];

    // Update world matrices
    this.chain.base.updateMatrixWorld(true);

    const endPos = this.chain.getEndEffectorPosition();

    // Get joint world position
    const jointPos = new THREE.Vector3();
    joint.mesh.getWorldPosition(jointPos);

    // Vector from joint to end effector
    const toEnd = endPos.clone().sub(jointPos);

    // Vector from joint to target
    const toTarget = target.clone().sub(jointPos);

    // Get the rotation axis in world space
    const axisLocal = new THREE.Vector3(
      joint.axis === "x" ? 1 : 0,
      joint.axis === "y" ? 1 : 0,
      joint.axis === "z" ? 1 : 0,
    );

    // Get parent's world matrix to transform axis
    const parentWorldMatrix = new THREE.Matrix4();
    if (joint.group.parent) {
      parentWorldMatrix.copy(joint.group.parent.matrixWorld);
    }

    // Extract rotation from parent matrix
    const parentQuat = new THREE.Quaternion();
    parentWorldMatrix.decompose(
      new THREE.Vector3(),
      parentQuat,
      new THREE.Vector3(),
    );

    const axisWorld = axisLocal.clone().applyQuaternion(parentQuat).normalize();

    // Project vectors onto plane perpendicular to rotation axis
    const toEndOnPlane = toEnd
      .clone()
      .sub(axisWorld.clone().multiplyScalar(toEnd.dot(axisWorld)));
    const toTargetOnPlane = toTarget
      .clone()
      .sub(axisWorld.clone().multiplyScalar(toTarget.dot(axisWorld)));

    // Skip if projections are too small
    const endLen = toEndOnPlane.length();
    const targetLen = toTargetOnPlane.length();

    if (endLen < 0.01 || targetLen < 0.01) {
      return;
    }

    toEndOnPlane.normalize();
    toTargetOnPlane.normalize();

    // Calculate angle between projections
    const dot = Math.max(-1, Math.min(1, toEndOnPlane.dot(toTargetOnPlane)));
    let angle = Math.acos(dot);

    // Determine sign using cross product
    const cross = new THREE.Vector3().crossVectors(
      toEndOnPlane,
      toTargetOnPlane,
    );
    if (cross.dot(axisWorld) < 0) {
      angle = -angle;
    }

    // Apply angle change with damping for smooth motion
    const newAngle = joint.getAngleRadians() + angle * this.damping;
    joint.setAngleRadians(newAngle);
  }
}
