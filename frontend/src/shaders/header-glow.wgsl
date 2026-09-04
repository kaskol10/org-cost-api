@fragment fn fs_main(@location(0) uv: vec2f) -> @location(0) vec4f {
  let center = vec2f(0.12, -0.15);
  let d = length(uv - center);
  let glow = exp(-3.2 * d);
  let accent = vec3f(0.24, 0.84, 0.78);
  let deep = vec3f(0.04, 0.06, 0.08);
  let color = mix(deep, accent, glow * 0.35);
  return vec4f(color, 1.0);
}
