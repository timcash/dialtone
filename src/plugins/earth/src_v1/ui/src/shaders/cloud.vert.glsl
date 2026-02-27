varying vec3 vPosition;
varying vec3 vNormal;
void main() {
    vPosition = position;
    vNormal = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
