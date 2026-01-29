varying vec3 vNormal;
varying vec3 vPosition;
void main() {
    vNormal = normalize(mat3(modelMatrix) * normal);
    vPosition = position;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
