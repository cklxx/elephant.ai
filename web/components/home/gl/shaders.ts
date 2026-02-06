/**
 * GLSL shaders for the particle field.
 *
 * Vertex shader: positions instanced particles using per-instance
 * offset + scale attributes, applies point sizing based on distance.
 *
 * Fragment shader: renders a soft radial glow with per-instance color.
 */

export const particleVertexShader = /* glsl */ `
  attribute vec3 instanceOffset;
  attribute float instanceScale;
  attribute vec3 instanceColor;

  varying vec3 vColor;
  varying float vAlpha;

  void main() {
    vColor = instanceColor;

    vec4 mvPosition = modelViewMatrix * vec4(instanceOffset, 1.0);
    float dist = -mvPosition.z;

    // Size attenuation: closer = larger
    float size = instanceScale * (300.0 / dist);
    gl_PointSize = clamp(size, 1.0, 12.0);

    // Fade out particles far from camera
    vAlpha = smoothstep(80.0, 20.0, dist);

    gl_Position = projectionMatrix * mvPosition;
  }
`;

export const particleFragmentShader = /* glsl */ `
  varying vec3 vColor;
  varying float vAlpha;

  void main() {
    // Soft radial glow
    float d = length(gl_PointCoord - vec2(0.5));
    float strength = 1.0 - smoothstep(0.0, 0.5, d);
    strength *= strength; // quadratic falloff

    if (strength < 0.01) discard;

    gl_FragColor = vec4(vColor, strength * vAlpha * 0.7);
  }
`;

/** Line shader: simple fading connection lines between particles. */
export const lineVertexShader = /* glsl */ `
  attribute vec3 color;
  varying vec3 vColor;

  void main() {
    vColor = color;
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`;

export const lineFragmentShader = /* glsl */ `
  varying vec3 vColor;

  void main() {
    gl_FragColor = vec4(vColor, 0.15);
  }
`;
