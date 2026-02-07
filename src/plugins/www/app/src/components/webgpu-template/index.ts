import { FpsCounter } from "../util/fps";
import { VisibilityMixin } from "../util/section";
import { startTyping } from "../util/typing";
import { setupWebGpuTemplateMenu } from "./menu";


/**
 * WebGPU template: minimal working section using the WebGPU API (no Three.js).
 * Use this as the starting point for new WebGPU-based sections.
 * Shows a lit sphere, WGSL shaders, adapter/device/context setup, and dispose/setVisible contract.
 */

type Mat4 = Float32Array;

const FLOATS_PER_MAT4 = 16;
const UNIFORM_FLOATS = FLOATS_PER_MAT4 * 3 + 4;

const createMat4 = (): Mat4 => new Float32Array(FLOATS_PER_MAT4);

const mat4Multiply = (out: Mat4, a: Mat4, b: Mat4): Mat4 => {
  const a00 = a[0];
  const a01 = a[1];
  const a02 = a[2];
  const a03 = a[3];
  const a10 = a[4];
  const a11 = a[5];
  const a12 = a[6];
  const a13 = a[7];
  const a20 = a[8];
  const a21 = a[9];
  const a22 = a[10];
  const a23 = a[11];
  const a30 = a[12];
  const a31 = a[13];
  const a32 = a[14];
  const a33 = a[15];

  const b00 = b[0];
  const b01 = b[1];
  const b02 = b[2];
  const b03 = b[3];
  const b10 = b[4];
  const b11 = b[5];
  const b12 = b[6];
  const b13 = b[7];
  const b20 = b[8];
  const b21 = b[9];
  const b22 = b[10];
  const b23 = b[11];
  const b30 = b[12];
  const b31 = b[13];
  const b32 = b[14];
  const b33 = b[15];

  out[0] = a00 * b00 + a10 * b01 + a20 * b02 + a30 * b03;
  out[1] = a01 * b00 + a11 * b01 + a21 * b02 + a31 * b03;
  out[2] = a02 * b00 + a12 * b01 + a22 * b02 + a32 * b03;
  out[3] = a03 * b00 + a13 * b01 + a23 * b02 + a33 * b03;
  out[4] = a00 * b10 + a10 * b11 + a20 * b12 + a30 * b13;
  out[5] = a01 * b10 + a11 * b11 + a21 * b12 + a31 * b13;
  out[6] = a02 * b10 + a12 * b11 + a22 * b12 + a32 * b13;
  out[7] = a03 * b10 + a13 * b11 + a23 * b12 + a33 * b13;
  out[8] = a00 * b20 + a10 * b21 + a20 * b22 + a30 * b23;
  out[9] = a01 * b20 + a11 * b21 + a21 * b22 + a31 * b23;
  out[10] = a02 * b20 + a12 * b21 + a22 * b22 + a32 * b23;
  out[11] = a03 * b20 + a13 * b21 + a23 * b22 + a33 * b23;
  out[12] = a00 * b30 + a10 * b31 + a20 * b32 + a30 * b33;
  out[13] = a01 * b30 + a11 * b31 + a21 * b32 + a31 * b33;
  out[14] = a02 * b30 + a12 * b31 + a22 * b32 + a32 * b33;
  out[15] = a03 * b30 + a13 * b31 + a23 * b32 + a33 * b33;
  return out;
};

const mat4Perspective = (
  out: Mat4,
  fovy: number,
  aspect: number,
  near: number,
  far: number,
): Mat4 => {
  const f = 1.0 / Math.tan(fovy / 2);
  const nf = 1 / (near - far);

  out[0] = f / aspect;
  out[1] = 0;
  out[2] = 0;
  out[3] = 0;
  out[4] = 0;
  out[5] = f;
  out[6] = 0;
  out[7] = 0;
  out[8] = 0;
  out[9] = 0;
  out[10] = (far + near) * nf;
  out[11] = -1;
  out[12] = 0;
  out[13] = 0;
  out[14] = 2 * far * near * nf;
  out[15] = 0;
  return out;
};

const mat4LookAt = (
  out: Mat4,
  eye: [number, number, number],
  center: [number, number, number],
  up: [number, number, number],
): Mat4 => {
  const [ex, ey, ez] = eye;
  const [cx, cy, cz] = center;

  let zx = ex - cx;
  let zy = ey - cy;
  let zz = ez - cz;
  let len = Math.hypot(zx, zy, zz);
  if (len === 0) {
    zz = 1;
    len = 1;
  }
  zx /= len;
  zy /= len;
  zz /= len;

  let xx = up[1] * zz - up[2] * zy;
  let xy = up[2] * zx - up[0] * zz;
  let xz = up[0] * zy - up[1] * zx;
  len = Math.hypot(xx, xy, xz);
  if (len === 0) {
    xx = 1;
    len = 1;
  }
  xx /= len;
  xy /= len;
  xz /= len;

  const yx = zy * xz - zz * xy;
  const yy = zz * xx - zx * xz;
  const yz = zx * xy - zy * xx;

  out[0] = xx;
  out[1] = yx;
  out[2] = zx;
  out[3] = 0;
  out[4] = xy;
  out[5] = yy;
  out[6] = zy;
  out[7] = 0;
  out[8] = xz;
  out[9] = yz;
  out[10] = zz;
  out[11] = 0;
  out[12] = -(xx * ex + xy * ey + xz * ez);
  out[13] = -(yx * ex + yy * ey + yz * ez);
  out[14] = -(zx * ex + zy * ey + zz * ez);
  out[15] = 1;
  return out;
};

const mat4RotationY = (out: Mat4, angle: number): Mat4 => {
  const c = Math.cos(angle);
  const s = Math.sin(angle);
  out[0] = c;
  out[1] = 0;
  out[2] = -s;
  out[3] = 0;
  out[4] = 0;
  out[5] = 1;
  out[6] = 0;
  out[7] = 0;
  out[8] = s;
  out[9] = 0;
  out[10] = c;
  out[11] = 0;
  out[12] = 0;
  out[13] = 0;
  out[14] = 0;
  out[15] = 1;
  return out;
};

const createSphereGeometry = (
  radius: number,
  latSegments: number,
  lonSegments: number,
) => {
  const vertices: number[] = [];
  const indices: number[] = [];

  for (let lat = 0; lat <= latSegments; lat++) {
    const theta = (lat * Math.PI) / latSegments;
    const sinTheta = Math.sin(theta);
    const cosTheta = Math.cos(theta);

    for (let lon = 0; lon <= lonSegments; lon++) {
      const phi = (lon * Math.PI * 2) / lonSegments;
      const sinPhi = Math.sin(phi);
      const cosPhi = Math.cos(phi);

      const x = sinTheta * cosPhi;
      const y = cosTheta;
      const z = sinTheta * sinPhi;

      vertices.push(radius * x, radius * y, radius * z);
      vertices.push(x, y, z);
    }
  }

  const stride = lonSegments + 1;
  for (let lat = 0; lat < latSegments; lat++) {
    for (let lon = 0; lon < lonSegments; lon++) {
      const a = lat * stride + lon;
      const b = a + stride;
      indices.push(a, b, a + 1);
      indices.push(b, b + 1, a + 1);
    }
  }

  return {
    vertices: new Float32Array(vertices),
    indices: new Uint32Array(indices),
  };
};

class WebGpuVisualization {
  container: HTMLElement;
  canvas: HTMLCanvasElement;
  device: GPUDevice;
  context: GPUCanvasContext;
  format: GPUTextureFormat;
  pipeline: GPURenderPipeline;
  uniformBuffer: GPUBuffer;
  bindGroup: GPUBindGroup;
  vertexBuffer: GPUBuffer;
  indexBuffer: GPUBuffer;
  indexCount: number;
  depthTexture?: GPUTexture;
  resizeObserver?: ResizeObserver;
  frameId = 0;
  time = 0;
  spinSpeed = 1;
  lastTime = performance.now();
  isVisible = true;
  frameCount = 0;
  private fpsCounter = new FpsCounter("webgpu-template");

  uniformData = new Float32Array(UNIFORM_FLOATS);
  model = createMat4();
  view = createMat4();
  projection = createMat4();
  mvp = createMat4();

  constructor(
    container: HTMLElement,
    canvas: HTMLCanvasElement,
    device: GPUDevice,
    context: GPUCanvasContext,
    format: GPUTextureFormat,
  ) {
    this.container = container;
    this.canvas = canvas;
    this.device = device;
    this.context = context;
    this.format = format;

    this.canvas.style.width = "100%";
    this.canvas.style.height = "100%";
    this.canvas.style.display = "block";
    this.canvas.style.position = "absolute";
    this.canvas.style.inset = "0";

    this.container.style.position = "relative";

    this.context.configure({
      device: this.device,
      format: this.format,
      alphaMode: "opaque",
    });

    const geometry = createSphereGeometry(1.0, 40, 40);
    this.vertexBuffer = this.device.createBuffer({
      size: geometry.vertices.byteLength,
      usage: GPUBufferUsage.VERTEX | GPUBufferUsage.COPY_DST,
    });
    this.device.queue.writeBuffer(this.vertexBuffer, 0, geometry.vertices);

    this.indexBuffer = this.device.createBuffer({
      size: geometry.indices.byteLength,
      usage: GPUBufferUsage.INDEX | GPUBufferUsage.COPY_DST,
    });
    this.device.queue.writeBuffer(this.indexBuffer, 0, geometry.indices);
    this.indexCount = geometry.indices.length;

    this.uniformBuffer = this.device.createBuffer({
      size: this.uniformData.byteLength,
      usage: GPUBufferUsage.UNIFORM | GPUBufferUsage.COPY_DST,
    });

    const shader = this.device.createShaderModule({
      code: `
        struct Uniforms {
          mvp: mat4x4<f32>,
          model: mat4x4<f32>,
          normal: mat4x4<f32>,
          lightPos: vec3<f32>,
          _pad: f32,
        };

        @group(0) @binding(0) var<uniform> uniforms: Uniforms;

        struct VertexInput {
          @location(0) position: vec3<f32>,
          @location(1) normal: vec3<f32>,
        };

        struct VertexOutput {
          @builtin(position) position: vec4<f32>,
          @location(0) worldPos: vec3<f32>,
          @location(1) normal: vec3<f32>,
        };

        @vertex
        fn vs_main(input: VertexInput) -> VertexOutput {
          var output: VertexOutput;
          let world = uniforms.model * vec4<f32>(input.position, 1.0);
          output.position = uniforms.mvp * vec4<f32>(input.position, 1.0);
          output.worldPos = world.xyz;
          output.normal = (uniforms.normal * vec4<f32>(input.normal, 0.0)).xyz;
          return output;
        }

        @fragment
        fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
          let lightDir = normalize(uniforms.lightPos - input.worldPos);
          let n = normalize(input.normal);
          let diffuse = max(dot(n, lightDir), 0.0);
          let base = vec3<f32>(0.25, 0.6, 1.0);
          let ambient = 0.18;
          let color = base * (ambient + diffuse);
          return vec4<f32>(color, 1.0);
        }
      `,
    });

    this.pipeline = this.device.createRenderPipeline({
      layout: "auto",
      vertex: {
        module: shader,
        entryPoint: "vs_main",
        buffers: [
          {
            arrayStride: 24,
            attributes: [
              { shaderLocation: 0, offset: 0, format: "float32x3" },
              { shaderLocation: 1, offset: 12, format: "float32x3" },
            ],
          },
        ],
      },
      fragment: {
        module: shader,
        entryPoint: "fs_main",
        targets: [{ format: this.format }],
      },
      primitive: {
        topology: "triangle-list",
        cullMode: "back",
      },
      depthStencil: {
        format: "depth24plus",
        depthWriteEnabled: true,
        depthCompare: "less",
      },
    });

    this.bindGroup = this.device.createBindGroup({
      layout: this.pipeline.getBindGroupLayout(0),
      entries: [{ binding: 0, resource: { buffer: this.uniformBuffer } }],
    });

    this.resize();
    this.animate();

    if (typeof ResizeObserver !== "undefined") {
      this.resizeObserver = new ResizeObserver(() => this.resize());
      this.resizeObserver.observe(this.container);
    } else {
      window.addEventListener("resize", this.resize);
    }
  }

  resize = () => {
    const rect = this.container.getBoundingClientRect();
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const width = Math.max(1, Math.floor(rect.width * dpr));
    const height = Math.max(1, Math.floor(rect.height * dpr));

    if (this.canvas.width !== width || this.canvas.height !== height) {
      this.canvas.width = width;
      this.canvas.height = height;
      this.context.configure({
        device: this.device,
        format: this.format,
        alphaMode: "opaque",
      });
      this.depthTexture?.destroy();
      this.depthTexture = this.device.createTexture({
        size: [width, height, 1],
        format: "depth24plus",
        usage: GPUTextureUsage.RENDER_ATTACHMENT,
      });
    }
  };

  updateUniforms() {
    const aspect = this.canvas.width / this.canvas.height;
    mat4Perspective(this.projection, Math.PI / 4, aspect, 0.1, 50);
    mat4LookAt(this.view, [0, 0.5, 4], [0, 0, 0], [0, 1, 0]);
    mat4RotationY(this.model, this.time * 0.35);

    const viewModel = createMat4();
    mat4Multiply(viewModel, this.view, this.model);
    mat4Multiply(this.mvp, this.projection, viewModel);

    this.uniformData.set(this.mvp, 0);
    this.uniformData.set(this.model, FLOATS_PER_MAT4);
    this.uniformData.set(this.model, FLOATS_PER_MAT4 * 2);

    const lightRadius = 3.0;
    const lightY = 1.5;
    const lightX = Math.cos(this.time) * lightRadius;
    const lightZ = Math.sin(this.time) * lightRadius;
    const lightOffset = FLOATS_PER_MAT4 * 3;
    this.uniformData[lightOffset] = lightX;
    this.uniformData[lightOffset + 1] = lightY;
    this.uniformData[lightOffset + 2] = lightZ;
    this.uniformData[lightOffset + 3] = 1;

    this.device.queue.writeBuffer(
      this.uniformBuffer,
      0,
      this.uniformData.buffer,
      this.uniformData.byteOffset,
      this.uniformData.byteLength,
    );
  }

  render() {
    if (!this.depthTexture) return;
    const commandEncoder = this.device.createCommandEncoder();
    const renderPass = commandEncoder.beginRenderPass({
      colorAttachments: [
        {
          view: this.context.getCurrentTexture().createView(),
          clearValue: { r: 0.02, g: 0.04, b: 0.08, a: 1 },
          loadOp: "clear",
          storeOp: "store",
        },
      ],
      depthStencilAttachment: {
        view: this.depthTexture.createView(),
        depthClearValue: 1.0,
        depthLoadOp: "clear",
        depthStoreOp: "store",
      },
    });

    renderPass.setPipeline(this.pipeline);
    renderPass.setBindGroup(0, this.bindGroup);
    renderPass.setVertexBuffer(0, this.vertexBuffer);
    renderPass.setIndexBuffer(this.indexBuffer, "uint32");
    renderPass.drawIndexed(this.indexCount);
    renderPass.end();

    this.device.queue.submit([commandEncoder.finish()]);
  }

  setVisible(visible: boolean) {
    VisibilityMixin.setVisible(this, visible, "webgpu-template");
    if (!visible) {
      this.fpsCounter.clear();
    }
  }

  animate = () => {
    this.frameId = requestAnimationFrame(this.animate);
    const now = performance.now();
    const delta = (now - this.lastTime) / 1000;
    this.lastTime = now;

    if (!this.isVisible) return;
    this.frameCount++;
    this.time += delta * this.spinSpeed;

    const cpuStart = performance.now();
    this.updateUniforms();
    const renderStart = performance.now();
    this.render();
    const renderMs = performance.now() - renderStart;
    const cpuMs = performance.now() - cpuStart;
    this.fpsCounter.tick(cpuMs, renderMs);
  };

  dispose() {
    cancelAnimationFrame(this.frameId);
    this.resizeObserver?.disconnect();
    window.removeEventListener("resize", this.resize);
    this.depthTexture?.destroy();
    this.vertexBuffer.destroy();
    this.indexBuffer.destroy();
    this.uniformBuffer.destroy();
    this.container.removeChild(this.canvas);
  }
}

export async function mountWebgpuTemplate(container: HTMLElement) {
  let stopTyping = () => { };
  container.innerHTML = `
    <div class="marketing-overlay" aria-label="WebGPU template section">
      <h2>Start here for WebGPU</h2>
      <p data-typing-subtitle></p>
    </div>
  `;

  // Initial menu (before async GPU init)
  let menu = setupWebGpuTemplateMenu({
    speed: 1,
    onSpeedChange: () => { },
  });

  const subtitleEl = container.querySelector(
    "[data-typing-subtitle]"
  ) as HTMLParagraphElement | null;
  const subtitles = [
    "Adapter, device, context, pipeline, and a lit sphere.",
    "Copy this component for new WebGPU sections.",
    "The simplest working WebGPU template.",
  ];
  stopTyping = startTyping(subtitleEl, subtitles);

  const canvas = document.createElement("canvas");
  canvas.className = "webgpu-canvas";
  container.appendChild(canvas);

  if (!("gpu" in navigator)) {
    stopTyping();
    container.innerHTML = `
      <div class="marketing-overlay" aria-label="WebGPU unsupported">
        <h2>WebGPU not available</h2>
        <p>Enable WebGPU in your browser to view this visualization.</p>
      </div>
    `;
    return {
      dispose: () => {
        menu.dispose();
        container.innerHTML = "";
      },
      setVisible: () => { },
    };
  }

  const adapter = await navigator.gpu.requestAdapter();
  if (!adapter) {
    container.innerHTML = `
      <div class="marketing-overlay" aria-label="WebGPU unavailable">
        <h2>WebGPU adapter missing</h2>
        <p>Unable to initialize a GPU adapter for this device.</p>
      </div>
    `;
    return {
      dispose: () => {
        menu.dispose();
        container.innerHTML = "";
      },
      setVisible: () => { },
    };
  }

  const device = await adapter.requestDevice();
  const context = canvas.getContext("webgpu");
  if (!context) {
    container.innerHTML = `
      <div class="marketing-overlay" aria-label="WebGPU context unavailable">
        <h2>WebGPU context failed</h2>
        <p>This browser could not create a WebGPU context.</p>
      </div>
    `;
    return {
      dispose: () => {
        menu.dispose();
        container.innerHTML = "";
      },
      setVisible: () => { },
    };
  }

  const viz = new WebGpuVisualization(
    container,
    canvas,
    device,
    context,
    navigator.gpu.getPreferredCanvasFormat(),
  );

  // Re-create menu linked to viz
  menu.dispose();
  menu = setupWebGpuTemplateMenu({
    speed: viz.spinSpeed,
    onSpeedChange: (value: number) => {
      viz.spinSpeed = value;
    },
  });

  return {
    dispose: () => {
      viz.dispose();
      menu.dispose();
      stopTyping();
      container.innerHTML = "";
    },
    setVisible: (visible: boolean) => {
      viz.setVisible(visible);
      menu.setToggleVisible(visible);
    },
  };
}
