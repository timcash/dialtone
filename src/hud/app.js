import * as THREE from "three";
// h3-js is loaded as a classic script (vendored UMD build) and exposes window.h3.
const { cellToBoundary, latLngToCell, gridDisk } = window.h3;

// Peer-supplied text is shown in the HUD; strip markup characters from every string
// in the state before anything renders it.
function scrubStrings(value) {
  if (typeof value === "string") return value.replace(/[<>"'`&]/g, "");
  if (Array.isArray(value)) return value.map(scrubStrings);
  if (value && typeof value === "object") {
    for (const k of Object.keys(value)) value[k] = scrubStrings(value[k]);
  }
  return value;
}
const INITIAL_STATE = scrubStrings(JSON.parse(document.getElementById("initial-state").textContent));
const LAND_H3_BITSET_B64 = JSON.parse(document.getElementById("land-cells").textContent);

// ---------------------------------------------------------------------------
// 0. deployrobotic Zoom Layer Stack (Layers 1..5) & State
// ---------------------------------------------------------------------------
const ZOOM_LAYERS = [
  { layer: 1, id: "globe",     name: "GLOBE",     camDist: 9.2,  desc: "Planetary H3 mesh & great-circle QUIC topology" },
  { layer: 2, id: "region",    name: "REGION",    camDist: 7.4,  desc: "Regional H3 resolution-3 cells & peer RTT links" },
  { layer: 3, id: "site",      name: "SITE",      camDist: 5.95, desc: "Tactical node symbol & 3-sector CPU/MEM/DSK gauges" },
  { layer: 4, id: "container", name: "CONTAINER", camDist: 4.9,  desc: "30s rolling variability compass & thrashing detector" },
  { layer: 5, id: "process",   name: "PROCESS",   camDist: 4.15, desc: "Supervised jobs, beat sequence & QUIC sockets" }
];
const PREVIOUS_LAYER_OPACITY = 0.25;

function clamp01(v) {
  return Math.min(1, Math.max(0, v));
}

function zoomLayerOpacity(layer, cameraLayer, part = "outline") {
  const above = cameraLayer - layer;
  if (above < 0) return clamp01(1 + above);
  if (part === "detail") return clamp01(1 - above * 0.45);
  if (above <= 1) return 1 + (PREVIOUS_LAYER_OPACITY - 1) * above;
  return PREVIOUS_LAYER_OPACITY * clamp01(2.6 - above * 0.5);
}

let meshState = INITIAL_STATE;
let selectedNode = INITIAL_STATE.selected_peer || INITIAL_STATE.self_node || "node-alpha";
let userDragOffset = new THREE.Quaternion();
let dragging = false;
let pointerDownPos = { x: 0, y: 0 };
let lastMouse = { x: 0, y: 0 };

// Tour & Layer state (camera stays zoomed out at Layer 1 GLOBE during tour; only users trigger zoom via arrow keys or double-click)
let tourPlaying = true;
let layerMode = 1; // 1..5 (user-controlled zoom layer)
let currentZoomLayer = 1;
let activeScanRing = 1; // 1..4 in-place concentric ring scan phase while dwelling on a node
let continuousCameraLayer = 1.0;
let targetCameraDist = 9.2;
let currentCameraDist = 9.2;
let tourStopIndex = 0;
let tourStops = [];

// Configurable slider parameters
let cfgTourDwellMs = 8000;
let cfgTourFlightMs = 1800;
let cfgFactoryScale = 1.0;
let cfgLandInk = 1.0;
let cfgCloudOpacity = 0.5;
let cfgPacketSpeed = 1.0;
let cfgShowLand = true;
let cfgShowArcs = true;
let cfgShowClouds = true;
let cfgShowLabels = true;
let cfgShowActivity = true;

const WARN_CPU_PCT = 75.0;
const ERROR_CPU_PCT = 92.0;
const WARN_MEM_PCT = 78.0;
const ERROR_MEM_PCT = 92.0;
const WARN_DISK_PCT = 82.0;
const ERROR_DISK_PCT = 94.0;
const GLOBE_RADIUS = 2.8;

// ---------------------------------------------------------------------------
// Theme Tokens (deployrobotic greyscale + blue focus + yellow warn + red error)
// ---------------------------------------------------------------------------
function readThemePalette() {
  const styles = getComputedStyle(document.documentElement);
  const tok = (name, fb) => styles.getPropertyValue(name).trim() || fb;
  return {
    groundHex: tok("--ground", "#111111"),
    inkHex: tok("--ink", "#ececec"),
    ink2Hex: tok("--ink-2", "#9c9c9c"),
    lineHex: tok("--line", "#474747"),
    line2Hex: tok("--line-2", "#2e2e2e"),
    accentHex: tok("--accent", "#f5f5f5"),
    warnHex: tok("--warn", "#b5ae92"),
    redHex: tok("--red", "#ef4136"),
    blueHex: tok("--blue", "#3d9bff"),
    ground: new THREE.Color(tok("--ground", "#111111")),
    ink: new THREE.Color(tok("--ink", "#ececec")),
    ink2: new THREE.Color(tok("--ink-2", "#9c9c9c")),
    line: new THREE.Color(tok("--line", "#474747")),
    line2: new THREE.Color(tok("--line-2", "#2e2e2e")),
    accent: new THREE.Color(tok("--accent", "#f5f5f5")),
    warn: new THREE.Color(tok("--warn", "#b5ae92")),
    red: new THREE.Color(tok("--red", "#ef4136")),
    blue: new THREE.Color(tok("--blue", "#3d9bff"))
  };
}
let palette = readThemePalette();

// ---------------------------------------------------------------------------
// 1. Decode embedded 11,216 H3 resolution-3 land cells (from deployrobotic)
// ---------------------------------------------------------------------------
function decodeLandH3Cells(b64) {
  const bin = atob(b64);
  const cells = [];
  for (let byteIdx = 0; byteIdx < bin.length; byteIdx++) {
    const byte = bin.charCodeAt(byteIdx);
    if (byte === 0) continue;
    for (let bit = 0; bit < 8; bit++) {
      if ((byte & (1 << bit)) !== 0) {
        const idx = (byteIdx << 3) | bit;
        cells.push("83" + idx.toString(16).padStart(4, "0") + "fffffffff");
      }
    }
  }
  return cells;
}

const LAND_H3_CELLS = decodeLandH3Cells(LAND_H3_BITSET_B64);

// ---------------------------------------------------------------------------
// 2. Public IP Geolocation + RTT Latency Clustering (Server + Client Fallback)
// ---------------------------------------------------------------------------
function isGloballyRoutableIpv4(ipStr) {
  const parts = (ipStr || "").split(".").map(Number);
  if (parts.length !== 4 || parts.some(x => Number.isNaN(x) || x < 0 || x > 255)) return false;
  const [a, b, c] = parts;
  if (a === 0 || a === 10 || a === 127) return false;
  if (a === 169 && b === 254) return false;
  if (a === 172 && b >= 16 && b <= 31) return false;
  if (a === 192 && b === 168) return false;
  if (a === 100 && b >= 64 && b <= 127) return false;
  if (a === 198 && (b === 18 || b === 19)) return false;
  if (a === 192 && b === 0 && (c === 0 || c === 2)) return false;
  if (a === 198 && b === 51 && c === 100) return false;
  if (a === 203 && b === 0 && c === 113) return false;
  if (a >= 224) return false;
  return true;
}

function approxGeoFromPublicIpv4(ipStr) {
  const parts = ipStr.split(".").map(Number);
  const [a, b, c] = parts;
  let baseLat = 37.75, baseLng = -122.42, region = "US-West";
  if ([3, 4, 8, 12, 13, 15, 16, 17, 18, 20, 23, 24, 32, 34, 35, 38, 40, 44, 45, 47, 50, 52, 54].includes(a)) {
    baseLat = 37.62; baseLng = -122.38; region = "US-West";
  } else if ((a >= 63 && a <= 76) || (a >= 96 && a <= 100) || (a >= 104 && a <= 108) || (a >= 128 && a <= 174) || (a >= 184 && a <= 192) || (a >= 198 && a <= 209) || a === 216) {
    if (b < 128) { baseLat = 37.38; baseLng = -122.05; region = "US-Pacific"; }
    else { baseLat = 39.04; baseLng = -77.49; region = "US-East"; }
  } else if (a === 2 || a === 5 || a === 31 || a === 37 || a === 46 || (a >= 77 && a <= 95) || (a >= 109 && a <= 109) || (a >= 176 && a <= 185) || (a >= 193 && a <= 195) || (a >= 212 && a <= 213) || a === 217) {
    baseLat = 50.11; baseLng = 8.68; region = "EU-Central";
  } else if (a === 1 || a === 14 || a === 27 || a === 36 || a === 39 || a === 42 || a === 43 || a === 49 || (a >= 58 && a <= 61) || (a >= 101 && a <= 103) || (a >= 110 && a <= 126) || (a >= 175 && a <= 175) || (a >= 180 && a <= 183) || (a >= 202 && a <= 203) || (a >= 210 && a <= 211) || (a >= 218 && a <= 223)) {
    baseLat = 35.68; baseLng = 139.76; region = "APAC-East";
  }
  const latOff = ((b / 255) - 0.5) * 4.2;
  const lngOff = ((c / 255) - 0.5) * 6.4;
  return {
    lat: Math.max(-70, Math.min(72, baseLat + latOff)),
    lng: ((((baseLng + lngOff) + 180) % 360 + 360) % 360) - 180,
    label: `${region} (${a}.${b}.x.x)`,
    source: "public_ip_geo",
    public_ip: `${a}.${b}.x.x`,
    uncertainty_km: 50
  };
}

function extractPublicIpv4FromNode(n) {
  const candidates = [
    ...(n.observed_remote_addrs || []),
    ...((n.iroh_transport && n.iroh_transport.advertised_ip_addrs) || []),
    (n.iroh_transport && n.iroh_transport.bound_non_tailnet_ipv4) || ""
  ];
  for (const raw of candidates) {
    const m = String(raw).match(/(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/);
    if (m && isGloballyRoutableIpv4(m[1])) {
      return m[1];
    }
  }
  return null;
}

function relayAnchorGeo(n) {
  const urls = [
    ...((n.iroh_transport && n.iroh_transport.advertised_relay_urls) || []),
    ...((n.iroh_transport && n.iroh_transport.relay_urls) || [])
  ].map(u => String(u).toLowerCase());
  for (const r of urls) {
    if (r.includes("usw") || r.includes("west") || r.includes("sjc") || r.includes("lax")) {
      return { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
    }
    if (r.includes("use") || r.includes("east") || r.includes("iad") || r.includes("nyc")) {
      return { lat: 39.04, lng: -77.49, region: "US-East (use1)" };
    }
    if (r.includes("euw") || r.includes("euc") || r.includes("fra") || r.includes("ams")) {
      return { lat: 50.11, lng: 8.68, region: "EU-Central" };
    }
    if (r.includes("aps") || r.includes("apn") || r.includes("tok") || r.includes("sin")) {
      return { lat: 35.68, lng: 139.76, region: "APAC" };
    }
  }
  return { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
}

function fnvBearingRad(a, b) {
  let h = 0x811c9dc5;
  const s = `${a}->${b}`;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193) >>> 0;
  }
  return ((h & 0xffff) / 65535.0) * Math.PI * 2;
}

function lookupPairRtt(a, b, nodes, rttMatrix) {
  const k1 = `${a}<->${b}`, k2 = `${b}<->${a}`;
  if (typeof rttMatrix[k1] === "number" && rttMatrix[k1] > 0) return rttMatrix[k1];
  if (typeof rttMatrix[k2] === "number" && rttMatrix[k2] > 0) return rttMatrix[k2];
  const na = nodes.find(x => x.node === a);
  if (na && na.peer_rtt_ms && typeof na.peer_rtt_ms[b] === "number" && na.peer_rtt_ms[b] > 0) return na.peer_rtt_ms[b];
  const nb = nodes.find(x => x.node === b);
  if (nb && nb.peer_rtt_ms && typeof nb.peer_rtt_ms[a] === "number" && nb.peer_rtt_ms[a] > 0) return nb.peer_rtt_ms[a];
  return null;
}

function resolveNodeLocationsJS(nodes, rttMatrix = {}) {
  const out = new Map();
  for (const n of nodes) {
    if (n.geo_location && n.geo_location.source === "public_ip_geo" && Number.isFinite(n.geo_location.lat) && Number.isFinite(n.geo_location.lng)) {
      out.set(n.node, n.geo_location);
      continue;
    }
    const pubIp = extractPublicIpv4FromNode(n);
    if (pubIp) {
      out.set(n.node, approxGeoFromPublicIpv4(pubIp));
    }
  }

  const anchors = nodes.filter(n => out.has(n.node));
  const unplaced = nodes.filter(n => !out.has(n.node));

  if (anchors.length > 0) {
    for (const n of unplaced) {
      let bestAnchor = anchors[0];
      let bestRtt = 18.0;
      let wLat = 0, wLng = 0, wSum = 0;
      for (const anc of anchors) {
        const g = out.get(anc.node);
        const rtt = lookupPairRtt(n.node, anc.node, nodes, rttMatrix) ?? 20.0;
        const w = 1.0 / Math.max(1.0, rtt);
        wLat += g.lat * w;
        wLng += g.lng * w;
        wSum += w;
        if (rtt < bestRtt || bestAnchor.node === anchors[0].node) {
          bestRtt = rtt;
          bestAnchor = anc;
        }
      }
      const baseLat = wSum > 0 ? wLat / wSum : out.get(bestAnchor.node).lat;
      const baseLng = wSum > 0 ? wLng / wSum : out.get(bestAnchor.node).lng;
      const distDeg = Math.max(3.6, Math.min(28.0, 3.4 + bestRtt * 0.22));
      const bearing = fnvBearingRad(bestAnchor.node, n.node);
      const latRad = (baseLat * Math.PI) / 180;
      const lat = Math.max(-72, Math.min(72, baseLat + distDeg * Math.cos(bearing)));
      const lng = ((((baseLng + (distDeg * Math.sin(bearing)) / Math.max(0.25, Math.cos(latRad))) + 180) % 360 + 360) % 360) - 180;
      out.set(n.node, {
        lat,
        lng,
        label: `RTT Cluster ~${bestRtt.toFixed(1)}ms from ${bestAnchor.node}`,
        source: "latency_cluster",
        anchor_node: bestAnchor.node,
        anchor_rtt_ms: Number(bestRtt.toFixed(1)),
        uncertainty_km: Math.round(Math.max(25, Math.min(1800, bestRtt * 65)))
      });
    }
  } else {
    const sorted = [...unplaced].sort((a, b) => a.node.localeCompare(b.node));
    const base = sorted[0] ? relayAnchorGeo(sorted[0]) : { lat: 37.62, lng: -122.38, region: "US-West (usw1)" };
    sorted.forEach((n, idx) => {
      if (idx === 0) {
        out.set(n.node, {
          lat: base.lat,
          lng: base.lng,
          label: `Latency Cluster Seed (${base.region})`,
          source: "latency_cluster",
          anchor_node: n.node,
          anchor_rtt_ms: 0.0,
          uncertainty_km: 80
        });
      } else {
        const seedName = sorted[0].node;
        const rtt = lookupPairRtt(n.node, seedName, nodes, rttMatrix) ?? 16.0;
        const distDeg = Math.max(4.2, Math.min(26.0, 3.8 + rtt * 0.24));
        const bearing = fnvBearingRad(seedName, n.node) + idx * 2.15;
        const latRad = (base.lat * Math.PI) / 180;
        const lat = Math.max(-72, Math.min(72, base.lat + distDeg * Math.cos(bearing)));
        const lng = ((((base.lng + (distDeg * Math.sin(bearing)) / Math.max(0.25, Math.cos(latRad))) + 180) % 360 + 360) % 360) - 180;
        out.set(n.node, {
          lat,
          lng,
          label: `RTT Cluster ~${rtt.toFixed(1)}ms (${base.region})`,
          source: "latency_cluster",
          anchor_node: seedName,
          anchor_rtt_ms: Number(rtt.toFixed(1)),
          uncertainty_km: Math.round(Math.max(30, Math.min(1800, rtt * 65)))
        });
      }
    });
  }

  // Spread co-located nodes slightly so neighboring micro-factories don't overlap on the globe
  const entries = [...out.entries()];
  for (let i = 0; i < entries.length; i++) {
    for (let j = i + 1; j < entries.length; j++) {
      const [na, ga] = entries[i];
      const [nb, gb] = entries[j];
      const dLat = gb.lat - ga.lat;
      const dLng = gb.lng - ga.lng;
      const dist = Math.hypot(dLat, dLng);
      if (dist < 2.2) {
        const angle = fnvBearingRad(na, nb);
        const push = (2.2 - dist) * 0.55;
        gb.lat = Math.max(-72, Math.min(72, gb.lat + Math.cos(angle) * push));
        gb.lng = ((((gb.lng + Math.sin(angle) * push) + 180) % 360 + 360) % 360) - 180;
      }
    }
  }

  return out;
}

// ---------------------------------------------------------------------------
// 3. Node Severity Classification (Nominal = Greyscale, Warn = Yellow, Error = Red)
// ---------------------------------------------------------------------------
function classifyNodeHealth(n) {
  const hr = n.stats || {};
  const ss = n.summary_stats || null;
  const sup = n.processes || [];
  const errors = [];
  const warnings = [];

  if (ss && ss.thrashing_detected) errors.push("THRASHING");
  if ((hr.cpu_percent || 0) >= ERROR_CPU_PCT) errors.push("CPU CRIT");
  else if ((hr.cpu_percent || 0) >= WARN_CPU_PCT) warnings.push("HIGH CPU");

  if ((hr.mem_percent || 0) >= ERROR_MEM_PCT) errors.push("MEM CRIT");
  else if ((hr.mem_percent || 0) >= WARN_MEM_PCT) warnings.push("HIGH MEM");

  if ((hr.disk_percent || 0) >= ERROR_DISK_PCT) errors.push("DISK CRIT");
  else if ((hr.disk_percent || 0) >= WARN_DISK_PCT) warnings.push("HIGH DISK");

  for (const job of sup) {
    if (String(job.status || "").toLowerCase().includes("fail") || String(job.status || "").toLowerCase().includes("crash")) {
      errors.push(`JOB ${job.name} ERR`);
    } else if ((job.restart_count || 0) > 0) {
      warnings.push(`JOB ${job.name} RESTART(${job.restart_count})`);
    }
  }
  if (n.tailnet_used) warnings.push("VPN PATH");

  const severity = errors.length > 0 ? "error" : (warnings.length > 0 ? "warn" : "nominal");
  return { severity, errors, warnings };
}

// ---------------------------------------------------------------------------
// 4. H3 Three.js Globe & deployrobotic Hidden-Line Materials
// ---------------------------------------------------------------------------
function latLngToSurfaceVector(lat, lng, radius = GLOBE_RADIUS) {
  const phi = (lat * Math.PI) / 180;
  const theta = (-lng * Math.PI) / 180;
  return new THREE.Vector3(
    Math.cos(phi) * Math.cos(theta),
    Math.sin(phi),
    Math.cos(phi) * Math.sin(theta)
  ).multiplyScalar(radius);
}

function createOutlineMaterial(color, outlinePx = 1.6) {
  return new THREE.ShaderMaterial({
    side: THREE.BackSide,
    transparent: true,
    uniforms: {
      uColor: { value: new THREE.Color(color) },
      uOutlinePx: { value: outlinePx },
      uOpacity: { value: 1.0 },
      uResolution: { value: new THREE.Vector2(window.innerWidth, window.innerHeight) }
    },
    vertexShader: `
      uniform float uOutlinePx;
      uniform vec2 uResolution;
      void main() {
        vec4 clipPosition = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
        vec3 viewNormal = normalize(normalMatrix * normal);
        vec2 clipNormal = normalize((projectionMatrix * vec4(viewNormal, 0.0)).xy);
        vec2 offset = (clipNormal * uOutlinePx * 2.0) / max(uResolution, vec2(1.0));
        clipPosition.xy += offset * clipPosition.w;
        gl_Position = clipPosition;
      }
    `,
    fragmentShader: `
      uniform vec3 uColor;
      uniform float uOpacity;
      void main() {
        gl_FragColor = vec4(uColor, uOpacity);
      }
    `
  });
}

// Depth-only occluder so the page's dotted grid ground shows through the globe
function createOccluderMaterial() {
  return new THREE.MeshBasicMaterial({
    colorWrite: false,
    polygonOffset: true,
    polygonOffsetFactor: 1,
    polygonOffsetUnits: 1
  });
}

const cloudVertexShader = `
  varying vec3 vObjectNormal;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vObjectNormal = normalize(position);
    vec4 worldPosition = modelMatrix * vec4(position, 1.0);
    vWorldPosition = worldPosition.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * worldPosition;
  }
`;

const cloudFragmentShader = `
  uniform float uTime;
  uniform float uOpacity;
  varying vec3 vObjectNormal;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  float hash(vec3 p) {
    p = fract(p * 0.3183099 + vec3(0.1, 0.2, 0.3));
    p *= 17.0;
    return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
  }
  float noise(vec3 x) {
    vec3 i = floor(x);
    vec3 f = fract(x);
    f = f * f * (3.0 - 2.0 * f);
    return mix(
      mix(mix(hash(i + vec3(0,0,0)), hash(i + vec3(1,0,0)), f.x),
          mix(hash(i + vec3(0,1,0)), hash(i + vec3(1,1,0)), f.x), f.y),
      mix(mix(hash(i + vec3(0,0,1)), hash(i + vec3(1,0,1)), f.x),
          mix(hash(i + vec3(0,1,1)), hash(i + vec3(1,1,1)), f.x), f.y),
      f.z
    );
  }
  float fbm(vec3 p) {
    float value = 0.0;
    float amp = 0.5;
    for (int i = 0; i < 4; i++) {
      value += amp * noise(p);
      p = p * 2.03 + vec3(0.17, -0.11, 0.23);
      amp *= 0.5;
    }
    return value;
  }
  void main() {
    vec3 p = normalize(vObjectNormal);
    vec3 drift = vec3(uTime * 0.014, -uTime * 0.005, uTime * 0.010);
    float base = fbm(p * 3.4 + drift);
    float detail = fbm(p * 6.8 - drift * 1.4);
    float bands = sin((p.y * 8.5) + base * 2.6 + uTime * 0.04) * 0.5 + 0.5;
    float cloud = smoothstep(0.56, 0.79, base * 0.65 + detail * 0.35) * smoothstep(0.24, 0.86, bands);
    vec3 viewDir = normalize(cameraPosition - vWorldPosition);
    float limb = 1.0 - max(dot(normalize(vWorldNormal), viewDir), 0.0);
    float rimBoost = mix(0.85, 1.25, pow(limb, 1.5));
    float alpha = cloud * uOpacity * rimBoost;
    if (alpha < 0.015) discard;
    gl_FragColor = vec4(vec3(0.68), alpha);
  }
`;

const atmosphereVertexShader = `
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vec4 worldPosition = modelMatrix * vec4(position, 1.0);
    vWorldPosition = worldPosition.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * normal);
    gl_Position = projectionMatrix * viewMatrix * worldPosition;
  }
`;

const atmosphereFragmentShader = `
  uniform vec3 uColor;
  uniform float uStrength;
  varying vec3 vWorldNormal;
  varying vec3 vWorldPosition;
  void main() {
    vec3 viewDir = normalize(cameraPosition - vWorldPosition);
    float rim = 1.0 - abs(dot(normalize(vWorldNormal), viewDir));
    float glow = pow(clamp(rim, 0.0, 1.0), 2.9) * uStrength;
    if (glow < 0.01) discard;
    gl_FragColor = vec4(uColor, glow);
  }
`;

function createGraticuleGeometry(radius) {
  const positions = [];
  for (let lat = -75; lat <= 75; lat += 15) {
    for (let lng = -180; lng < 180; lng += 4) {
      const a = latLngToSurfaceVector(lat, lng, radius);
      const b = latLngToSurfaceVector(lat, lng + 4, radius);
      positions.push(a.x, a.y, a.z, b.x, b.y, b.z);
    }
  }
  for (let lng = -180; lng < 180; lng += 15) {
    for (let lat = -75; lat < 75; lat += 4) {
      const a = latLngToSurfaceVector(lat, lng, radius);
      const b = latLngToSurfaceVector(Math.min(75, lat + 4), lng, radius);
      positions.push(a.x, a.y, a.z, b.x, b.y, b.z);
    }
  }
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute("position", new THREE.Float32BufferAttribute(positions, 3));
  return geometry;
}

function createLandGeometry(radius, cells) {
  const surfacePositions = [];
  const outlinePositions = [];
  const surfaceRadius = radius * 1.002;
  const outlineRadius = radius * 1.003;

  for (const cell of cells) {
    const ring = cellToBoundary(cell, true);
    if (ring.length < 3) continue;
    const surfaceRing = [];
    const outlineRing = [];
    const center = new THREE.Vector3();

    for (const [lng, lat] of ring) {
      const surfacePoint = latLngToSurfaceVector(lat, lng, surfaceRadius);
      const outlinePoint = latLngToSurfaceVector(lat, lng, outlineRadius);
      surfaceRing.push(surfacePoint);
      outlineRing.push(outlinePoint);
      center.add(surfacePoint);
    }
    center.divideScalar(surfaceRing.length).normalize().multiplyScalar(surfaceRadius);

    for (let i = 0; i < surfaceRing.length; i++) {
      const curr = surfaceRing[i];
      const next = surfaceRing[(i + 1) % surfaceRing.length];
      surfacePositions.push(center.x, center.y, center.z, curr.x, curr.y, curr.z, next.x, next.y, next.z);
      const currOut = outlineRing[i];
      const nextOut = outlineRing[(i + 1) % outlineRing.length];
      outlinePositions.push(currOut.x, currOut.y, currOut.z, nextOut.x, nextOut.y, nextOut.z);
    }
  }

  const surface = new THREE.BufferGeometry();
  surface.setAttribute("position", new THREE.Float32BufferAttribute(surfacePositions, 3));
  surface.computeVertexNormals();

  const outlines = new THREE.BufferGeometry();
  outlines.setAttribute("position", new THREE.Float32BufferAttribute(outlinePositions, 3));
  return { surface, outlines };
}

function createLabelMaterial(title, subtitle, borderHex) {
  const c = document.createElement("canvas");
  c.width = 512;
  c.height = 128;
  const g = c.getContext("2d");

  g.shadowColor = "rgba(0, 0, 0, 0.96)";
  g.shadowBlur = 10;
  g.shadowOffsetX = 0;
  g.shadowOffsetY = 2;

  g.font = "700 36px 'B612 Mono', 'JetBrains Mono', ui-monospace, monospace";
  g.fillStyle = borderHex || palette.inkHex;
  g.textBaseline = "middle";
  g.fillText(title, 12, 44);

  g.font = "700 24px 'B612 Mono', 'JetBrains Mono', ui-monospace, monospace";
  g.fillStyle = palette.inkHex;
  g.fillText(subtitle, 12, 88);

  const texture = new THREE.CanvasTexture(c);
  texture.needsUpdate = true;
  return new THREE.SpriteMaterial({
    map: texture,
    transparent: true,
    depthTest: false,
    depthWrite: false
  });
}

// ---------------------------------------------------------------------------
// 5. Abstract Battlefield Symbology & Nested Concentric Rings Builder
// ---------------------------------------------------------------------------
function createTriangleLinePoints(radius, y = 0.04, rotOffset = 0) {
  const pts = [];
  for (let i = 0; i <= 3; i++) {
    const a = rotOffset + (i % 3) * ((Math.PI * 2) / 3);
    pts.push(new THREE.Vector3(Math.sin(a) * radius, y, -Math.cos(a) * radius));
  }
  return pts;
}

function createDiamondLinePoints(radius, y = 0.045) {
  return [
    new THREE.Vector3(0, y, -radius),
    new THREE.Vector3(radius, y, 0),
    new THREE.Vector3(0, y, radius),
    new THREE.Vector3(-radius, y, 0),
    new THREE.Vector3(0, y, -radius)
  ];
}

function createCircleArcPoints(radius, startAngle = 0, endAngle = Math.PI * 2, segments = 64, y = 0.04) {
  const pts = [];
  for (let i = 0; i <= segments; i++) {
    const a = startAngle + (i / segments) * (endAngle - startAngle);
    pts.push(new THREE.Vector3(Math.sin(a) * radius, y, -Math.cos(a) * radius));
  }
  return pts;
}

function createDashedCircleGeometry(radius, dashCount = 24, duty = 0.55, y = 0.04) {
  const pts = [];
  const step = (Math.PI * 2) / dashCount;
  for (let i = 0; i < dashCount; i++) {
    const a0 = i * step;
    const a1 = a0 + step * duty;
    pts.push(
      new THREE.Vector3(Math.sin(a0) * radius, y, -Math.cos(a0) * radius),
      new THREE.Vector3(Math.sin(a1) * radius, y, -Math.cos(a1) * radius)
    );
  }
  return new THREE.BufferGeometry().setFromPoints(pts);
}

function createTickRingGeometry(radius, tickCount = 24, tickLen = 0.08, y = 0.04) {
  const pts = [];
  for (let i = 0; i < tickCount; i++) {
    const a = (i / tickCount) * Math.PI * 2;
    const isCardinal = i % (tickCount / 4) === 0;
    const len = isCardinal ? tickLen * 1.65 : tickLen;
    const rIn = radius - len * 0.5;
    const rOut = radius + len * 0.5;
    pts.push(
      new THREE.Vector3(Math.sin(a) * rIn, y, -Math.cos(a) * rIn),
      new THREE.Vector3(Math.sin(a) * rOut, y, -Math.cos(a) * rOut)
    );
  }
  return new THREE.BufferGeometry().setFromPoints(pts);
}

function createSmallFilledTriangleGeometry(size) {
  const geom = new THREE.BufferGeometry();
  const r = size;
  const verts = new Float32Array([
    0, 0, -r * 1.15,
    -r * 0.88, 0, r * 0.7,
    r * 0.88, 0, r * 0.7
  ]);
  geom.setAttribute("position", new THREE.BufferAttribute(verts, 3));
  geom.computeVertexNormals();
  return geom;
}

function createCornerBracketsGeometry(halfSize, armLen, y = 0.03) {
  const s = halfSize;
  const a = armLen;
  const pts = [
    // Top-left
    new THREE.Vector3(-s, y, -s + a), new THREE.Vector3(-s, y, -s),
    new THREE.Vector3(-s, y, -s), new THREE.Vector3(-s + a, y, -s),
    // Top-right
    new THREE.Vector3(s - a, y, -s), new THREE.Vector3(s, y, -s),
    new THREE.Vector3(s, y, -s), new THREE.Vector3(s, y, -s + a),
    // Bottom-right
    new THREE.Vector3(s, y, s - a), new THREE.Vector3(s, y, s),
    new THREE.Vector3(s, y, s), new THREE.Vector3(s - a, y, s),
    // Bottom-left
    new THREE.Vector3(-s + a, y, s), new THREE.Vector3(-s, y, s),
    new THREE.Vector3(-s, y, s), new THREE.Vector3(-s, y, s - a)
  ];
  return new THREE.BufferGeometry().setFromPoints(pts);
}

function createMicroTagSprite(text, borderHex) {
  const canvas = document.createElement("canvas");
  canvas.width = 384;
  canvas.height = 72;
  const ctx = canvas.getContext("2d");

  ctx.shadowColor = "rgba(0, 0, 0, 0.96)";
  ctx.shadowBlur = 8;
  ctx.shadowOffsetX = 0;
  ctx.shadowOffsetY = 2;

  ctx.fillStyle = borderHex || palette.inkHex;
  ctx.font = "700 32px 'B612 Mono', 'JetBrains Mono', ui-monospace, monospace";
  ctx.textBaseline = "middle";
  ctx.textAlign = "center";
  ctx.fillText(text, 192, 36);

  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  const mat = new THREE.SpriteMaterial({
    map: tex,
    transparent: true,
    depthTest: false,
    depthWrite: false
  });
  const sprite = new THREE.Sprite(mat);
  sprite.scale.set(1.65, 0.32, 1);
  sprite.renderOrder = 18;
  return sprite;
}

function registerRingLayer(group, ringIdx, minLayer) {
  const tracked = [];
  group.traverse(obj => {
    if (obj.material) {
      obj.material.transparent = true;
      const baseOp = typeof obj.material.opacity === "number" ? obj.material.opacity : 1.0;
      tracked.push({ mat: obj.material, baseOpacity: baseOp });
    }
  });
  return { group, ringIdx, minLayer, tracked };
}

function createMicroFactoryIcon(nodeData, geo, isFocused, health) {
  const hr = nodeData.stats || {};
  const ss = nodeData.summary_stats || null;
  const cpuPct = Math.max(4, Math.min(100, hr.cpu_percent || 8));
  const memPct = Math.max(6, Math.min(100, hr.mem_percent || 25));
  const dskPct = Math.max(6, Math.min(100, hr.disk_percent || 25));
  const supCount = (nodeData.processes || nodeData.processes || []).length;

  // Semantic colors:
  // - Red strictly for errors
  // - Muted warm-grey yellow for warnings
  // - Pure monochrome greyscale (--ink / --ink-2 / --line) for nominal & focused elements
  const primaryColor = health.severity === "error"
    ? palette.red
    : (health.severity === "warn"
      ? palette.warn
      : (isFocused ? palette.accent : palette.ink));

  const secondaryColor = isFocused ? palette.accent : palette.ink2;
  const focusEdgeColor = health.severity === "error"
    ? palette.red
    : (health.severity === "warn" ? palette.warn : palette.accent);

  const root = new THREE.Group();
  const R_SURF = GLOBE_RADIUS * 1.004;
  const surfacePoint = latLngToSurfaceVector(geo.lat, geo.lng, R_SURF);
  const normal = surfacePoint.clone().normalize();
  let east = new THREE.Vector3(0, 1, 0).cross(normal);
  if (east.lengthSq() < 1e-4) east = new THREE.Vector3(1, 0, 0);
  east.normalize();
  // Proper right-handed tangent basis (det = +1): east × normal = south
  const south = east.clone().cross(normal).normalize();

  const basis = new THREE.Matrix4().makeBasis(east, normal, south);
  root.position.copy(surfacePoint);
  root.quaternion.setFromRotationMatrix(basis);

  const baseScale = 0.056 * cfgFactoryScale * (isFocused ? 1.32 : 0.72);
  root.scale.setScalar(baseScale);

  // Spherical curvature compensation so circles of any local radius r hug the globe surface flush
  const surfY = (rLocal, lift = 0.02) => {
    const dWorld = rLocal * baseScale;
    const dropWorld = Math.sqrt(Math.max(0.01, R_SURF * R_SURF - dWorld * dWorld)) - R_SURF;
    return dropWorld / baseScale + lift;
  };

  // Helper to create a small circle lying flat on the local globe surface at (cx, cz)
  const makeSmallSurfaceCircle = (cx, cz, radius, color, opacity = 0.92) => {
    const rCenter = Math.hypot(cx, cz);
    const y = surfY(rCenter, 0.025);
    const pts = [];
    const segs = 24;
    for (let i = 0; i <= segs; i++) {
      const a = (i / segs) * Math.PI * 2;
      const px = cx + Math.sin(a) * radius;
      const pz = cz - Math.cos(a) * radius;
      pts.push(new THREE.Vector3(px, surfY(Math.hypot(px, pz), 0.025), pz));
    }
    return new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(pts),
      new THREE.LineBasicMaterial({ color, transparent: true, opacity })
    );
  };

  const rotors = [];
  const leds = [];
  const nestedRings = [];

  // =========================================================================
  // LAYER 1 (CORE): Surface-Aligned Concentric Circles + Ping Circle
  // Pure circles flush with the globe surface — ZERO triangles or text
  // =========================================================================
  const centerDiscGeom = new THREE.CircleGeometry(isFocused ? 0.13 : 0.10, 28);
  centerDiscGeom.rotateX(-Math.PI * 0.5);
  const centerDot = new THREE.Mesh(
    centerDiscGeom,
    new THREE.MeshBasicMaterial({ color: primaryColor, side: THREE.DoubleSide })
  );
  centerDot.position.set(0, surfY(0, 0.022), 0);
  root.add(centerDot);

  const coreCirclePts = createCircleArcPoints(0.52, 0, Math.PI * 2, 56, surfY(0.52, 0.022));
  const coreCircle = new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(coreCirclePts),
    new THREE.LineBasicMaterial({
      color: primaryColor,
      transparent: true,
      opacity: isFocused ? 1.0 : 0.82
    })
  );
  root.add(coreCircle);

  if (isFocused) {
    const outerFocusPts = createCircleArcPoints(0.74, 0, Math.PI * 2, 64, surfY(0.74, 0.022));
    root.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(outerFocusPts),
        new THREE.LineBasicMaterial({
          color: focusEdgeColor,
          transparent: true,
          opacity: 0.68
        })
      )
    );
  }

  // Expanding circular ping ring (flush with globe surface)
  const pulseCirclePts = createCircleArcPoints(0.28, 0, Math.PI * 2, 48, surfY(0.28, 0.02));
  const pulseRing = new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(pulseCirclePts),
    new THREE.LineBasicMaterial({
      color: focusEdgeColor,
      transparent: true,
      opacity: isFocused ? 0.75 : 0.35
    })
  );
  root.add(pulseRing);

  // =========================================================================
  // NESTED RING 1 (radius r = 0.96):
  // Inner Circle Ring + Dashed Inner Circle + Peer RTT Circle Nodes
  // =========================================================================
  const ring1Group = new THREE.Group();
  const r1 = 0.96;

  ring1Group.add(
    new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r1, 0, Math.PI * 2, 64, surfY(r1, 0.022))),
      new THREE.LineBasicMaterial({
        color: isFocused ? palette.accent : palette.line,
        transparent: true,
        opacity: isFocused ? 0.85 : 0.48
      })
    )
  );

  const innerDashedCircle = new THREE.LineSegments(
    createDashedCircleGeometry(0.72, 16, 0.55, surfY(0.72, 0.022)),
    new THREE.LineBasicMaterial({
      color: secondaryColor,
      transparent: true,
      opacity: 0.72
    })
  );
  ring1Group.add(innerDashedCircle);
  rotors.push(innerDashedCircle);

  const peerEntries = Object.entries(nodeData.peer_rtt_ms || {});
  peerEntries.forEach(([_peerName, rttVal], pIdx) => {
    const rttNum = Number(rttVal) || 0;
    const pAngle = (pIdx / Math.max(1, peerEntries.length)) * Math.PI * 2 + 0.52;
    const pColor = rttNum >= 200 ? palette.warn : (isFocused ? palette.accent : palette.ink);
    const cx = Math.sin(pAngle) * r1;
    const cz = -Math.cos(pAngle) * r1;
    ring1Group.add(makeSmallSurfaceCircle(cx, cz, 0.085, pColor, 0.92));
  });

  root.add(ring1Group);
  nestedRings.push(registerRingLayer(ring1Group, 1, 1.4));

  // =========================================================================
  // NESTED RING 2 (radius r = 1.48):
  // 3-Sector Concentric Circular Resource Gauge Ring (CPU, MEM, DISK arcs)
  // =========================================================================
  const ring2Group = new THREE.Group();
  const r2 = 1.48;
  const sectorSpan = (Math.PI * 2) / 3;
  const gapRad = 0.24;

  const metrics = [
    { pct: cpuPct, start: -sectorSpan * 0.5 + gapRad * 0.5, warnAt: WARN_CPU_PCT, errAt: ERROR_CPU_PCT },
    { pct: memPct, start: sectorSpan * 0.5 + gapRad * 0.5,  warnAt: WARN_MEM_PCT, errAt: ERROR_MEM_PCT },
    { pct: dskPct, start: sectorSpan * 1.5 + gapRad * 0.5,  warnAt: WARN_DISK_PCT, errAt: ERROR_DISK_PCT }
  ];

  for (const m of metrics) {
    const usableSpan = sectorSpan - gapRad;
    const endTrack = m.start + usableSpan;
    const fillSpan = Math.max(0.06, (m.pct / 100) * usableSpan);
    const endFill = m.start + fillSpan;
    const midAngle = m.start + usableSpan * 0.5;

    const mColor = m.pct >= m.errAt
      ? palette.red
      : (m.pct >= m.warnAt ? palette.warn : (isFocused ? palette.accent : palette.ink));

    ring2Group.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r2, m.start, endTrack, 28, surfY(r2, 0.022))),
        new THREE.LineBasicMaterial({ color: palette.line, transparent: true, opacity: 0.45 })
      )
    );

    ring2Group.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r2, m.start, endFill, 28, surfY(r2, 0.025))),
        new THREE.LineBasicMaterial({ color: mColor, transparent: true, opacity: 0.96 })
      )
    );
    ring2Group.add(
      new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r2 + 0.055, m.start, endFill, 28, surfY(r2 + 0.055, 0.025))),
        new THREE.LineBasicMaterial({ color: mColor, transparent: true, opacity: 0.72 })
      )
    );

    const midCircle = makeSmallSurfaceCircle(
      Math.sin(midAngle) * (r2 + 0.14),
      -Math.cos(midAngle) * (r2 + 0.14),
      0.08,
      mColor,
      0.92
    );
    ring2Group.add(midCircle);
    leds.push(midCircle.material);
  }

  root.add(ring2Group);
  nestedRings.push(registerRingLayer(ring2Group, 2, 2.0));

  // =========================================================================
  // NESTED RING 3 (radius r = 2.04):
  // Concentric Variability Circle Ring + 4 Cardinal Small Circles
  // =========================================================================
  const ring3Group = new THREE.Group();
  const r3 = 2.04;

  ring3Group.add(
    new THREE.LineSegments(
      createDashedCircleGeometry(r3 - 0.10, 32, 0.55, surfY(r3 - 0.10, 0.022)),
      new THREE.LineBasicMaterial({ color: palette.line, transparent: true, opacity: 0.58 })
    )
  );
  ring3Group.add(
    new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r3, 0, Math.PI * 2, 72, surfY(r3, 0.022))),
      new THREE.LineBasicMaterial({ color: secondaryColor, transparent: true, opacity: 0.76 })
    )
  );

  const focusBracket = new THREE.LineSegments(
    createDashedCircleGeometry(r3 + 0.12, 12, 0.45, surfY(r3 + 0.12, 0.022)),
    new THREE.LineBasicMaterial({
      color: focusEdgeColor,
      transparent: true,
      opacity: 0.9
    })
  );
  ring3Group.add(focusBracket);

  const cpuStd = (ss && ss.cpu_pct && typeof ss.cpu_pct.stddev === "number") ? ss.cpu_pct.stddev : 0.4;
  const memStd = (ss && ss.mem_pct && typeof ss.mem_pct.stddev === "number") ? ss.mem_pct.stddev : 0.2;
  const thrashing = Boolean(ss && ss.thrashing_detected);

  const cardinalStats = [
    { angle: 0,               warn: cpuStd >= 18.0, err: false },
    { angle: Math.PI * 0.5,   warn: memStd >= 10.0, err: false },
    { angle: Math.PI,         warn: false,          err: false },
    { angle: Math.PI * 1.5,   warn: thrashing,      err: thrashing }
  ];

  for (const cs of cardinalStats) {
    const cColor = cs.err ? palette.red : (cs.warn ? palette.warn : (isFocused ? palette.accent : palette.ink));
    const cx = Math.sin(cs.angle) * r3;
    const cz = -Math.cos(cs.angle) * r3;
    ring3Group.add(makeSmallSurfaceCircle(cx, cz, 0.095, cColor, 0.95));
  }

  root.add(ring3Group);
  nestedRings.push(registerRingLayer(ring3Group, 3, 2.4));

  // =========================================================================
  // NESTED RING 4 (radius r = 2.62):
  // Outer Concentric Double-Circle + Orbiting Small Circles
  // =========================================================================
  const ring4Group = new THREE.Group();
  const r4 = 2.62;

  ring4Group.add(
    new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r4 - 0.06, 0, Math.PI * 2, 80, surfY(r4 - 0.06, 0.022))),
      new THREE.LineBasicMaterial({ color: palette.line, transparent: true, opacity: 0.55 })
    )
  );
  ring4Group.add(
    new THREE.Line(
      new THREE.BufferGeometry().setFromPoints(createCircleArcPoints(r4 + 0.06, 0, Math.PI * 2, 80, surfY(r4 + 0.06, 0.022))),
      new THREE.LineBasicMaterial({ color: focusEdgeColor, transparent: true, opacity: 0.82 })
    )
  );

  const droneOrbitGroup = new THREE.Group();
  const trackCount = Math.max(4, Math.min(8, 4 + supCount));
  for (let d = 0; d < trackCount; d++) {
    const angle = (d / trackCount) * Math.PI * 2;
    const cx = Math.cos(angle) * r4;
    const cz = Math.sin(angle) * r4;
    droneOrbitGroup.add(
      makeSmallSurfaceCircle(cx, cz, d % 2 === 0 ? 0.08 : 0.055, d % 2 === 0 ? focusEdgeColor : palette.ink, 0.92)
    );
  }
  ring4Group.add(droneOrbitGroup);

  root.add(ring4Group);
  nestedRings.push(registerRingLayer(ring4Group, 4, 2.75));

  // =========================================================================
  // Hit sphere (invisible raycast target — zero text in 3D scene)
  // =========================================================================
  const hitTarget = new THREE.Mesh(
    new THREE.SphereGeometry(2.4, 8, 8),
    new THREE.MeshBasicMaterial({ transparent: true, opacity: 0.0, depthWrite: false })
  );
  hitTarget.position.set(0, 0.2, 0);
  hitTarget.userData = { node: nodeData.node };
  root.add(hitTarget);

  const spireH = 0.18;
  const padH = 0.04;
  const labelSprite = null;

  // World-space anchor point for connecting great-circle QUIC arcs
  const spireWorldTip = surfacePoint.clone().addScaledVector(normal, 0.08 * baseScale);

  return {
    node: nodeData.node,
    geo,
    root,
    centerDot,
    hitTarget,
    labelSprite,
    rotors,
    leds,
    pulseRing,
    droneOrbitGroup,
    focusBracket,
    nestedRings,
    spireH,
    padH,
    cpuPct,
    isFocused,
    health,
    spireWorldTip
  };
}

// ---------------------------------------------------------------------------
// 6. Initialize Three.js Scene, Camera & H3 Globe Hierarchy
// ---------------------------------------------------------------------------
const canvas = document.getElementById("hud-3d-canvas");
const renderer = new THREE.WebGLRenderer({
  canvas,
  antialias: true,
  alpha: true,
  powerPreference: "high-performance"
});
renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
renderer.setSize(window.innerWidth, window.innerHeight, false);
renderer.setClearColor(0x000000, 0);

const scene = new THREE.Scene();
const camera = new THREE.PerspectiveCamera(34, window.innerWidth / window.innerHeight, 0.05, 100);
camera.position.set(0, 0, currentCameraDist);
camera.lookAt(0, 0, 0);

const globeRoot = new THREE.Group();
globeRoot.position.set(0, 0, 0);
scene.add(globeRoot);

const globeSurface = new THREE.Group();
globeRoot.add(globeSurface);

const sphereGeom = new THREE.SphereGeometry(GLOBE_RADIUS, 48, 36);
const oceanMesh = new THREE.Mesh(sphereGeom, createOccluderMaterial());
globeSurface.add(oceanMesh);

const limbMat = createOutlineMaterial(palette.ink2Hex, 1.6);
const limbMesh = new THREE.Mesh(sphereGeom, limbMat);
limbMesh.scale.setScalar(1.002);
globeSurface.add(limbMesh);

const graticuleMat = new THREE.LineBasicMaterial({
  color: palette.line,
  transparent: true,
  opacity: 0.42
});
const graticuleLines = new THREE.LineSegments(
  createGraticuleGeometry(GLOBE_RADIUS * 1.002),
  graticuleMat
);
globeSurface.add(graticuleLines);

const landGeom = createLandGeometry(GLOBE_RADIUS, LAND_H3_CELLS);
const continentsMat = new THREE.MeshBasicMaterial({
  color: palette.ink,
  transparent: true,
  opacity: 0.08,
  side: THREE.DoubleSide,
  depthWrite: false,
  polygonOffset: true,
  polygonOffsetFactor: 1,
  polygonOffsetUnits: 1
});
const continentsMesh = new THREE.Mesh(landGeom.surface, continentsMat);
globeSurface.add(continentsMesh);

const landOutlinesMat = new THREE.LineBasicMaterial({
  color: palette.ink2,
  transparent: true,
  opacity: 0.46
});
const landOutlines = new THREE.LineSegments(landGeom.outlines, landOutlinesMat);
globeSurface.add(landOutlines);

const cloudUniforms = {
  uTime: { value: 0 },
  uOpacity: { value: 0.12 }
};
const cloudsMesh = new THREE.Mesh(
  new THREE.SphereGeometry(GLOBE_RADIUS * 1.028, 40, 28),
  new THREE.ShaderMaterial({
    transparent: true,
    depthWrite: false,
    side: THREE.FrontSide,
    uniforms: cloudUniforms,
    vertexShader: cloudVertexShader,
    fragmentShader: cloudFragmentShader
  })
);
globeSurface.add(cloudsMesh);

const atmosphereMat = new THREE.ShaderMaterial({
  transparent: true,
  depthWrite: false,
  side: THREE.BackSide,
  blending: THREE.AdditiveBlending,
  uniforms: {
    uColor: { value: new THREE.Color(palette.ink2Hex) },
    uStrength: { value: 0.16 }
  },
  vertexShader: atmosphereVertexShader,
  fragmentShader: atmosphereFragmentShader
});
const atmosphereMesh = new THREE.Mesh(
  new THREE.SphereGeometry(GLOBE_RADIUS * 1.055, 40, 28),
  atmosphereMat
);
globeSurface.add(atmosphereMesh);

// Dynamic groups for H3 active cells, 3D micro-factories, and great-circle QUIC arcs
const activeH3Group = new THREE.Group();
const siteGroup = new THREE.Group();
const arcGroup = new THREE.Group();
globeSurface.add(activeH3Group);
globeSurface.add(siteGroup);
globeSurface.add(arcGroup);

let siteVisuals = [];
let arcVisuals = [];
let lastVisualSignature = "";

function clearGroup(group) {
  while (group.children.length > 0) {
    const child = group.children.pop();
    child.traverse(obj => {
      if (obj.geometry) obj.geometry.dispose();
    });
  }
}

function createGreatCircleArcPoints(vecA, vecB, segments = 44) {
  const va = vecA.clone().normalize();
  const vb = vecB.clone().normalize();
  const angle = va.angleTo(vb);
  const rA = vecA.length();
  const rB = vecB.length();
  const points = [];
  for (let i = 0; i <= segments; i++) {
    const t = i / segments;
    let dir;
    if (angle < 0.001) {
      dir = va.clone();
    } else {
      const sinTotal = Math.sin(angle);
      const wA = Math.sin((1 - t) * angle) / sinTotal;
      const wB = Math.sin(t * angle) / sinTotal;
      dir = va.clone().multiplyScalar(wA).add(vb.clone().multiplyScalar(wB)).normalize();
    }
    const baseR = rA * (1 - t) + rB * t;
    const archLift = Math.sin(t * Math.PI) * Math.min(0.45, 0.09 + angle * 0.24);
    points.push(dir.multiplyScalar(baseR + archLift));
  }
  return points;
}

function rebuildGlobeNodesAndArcs(force = false) {
  const nodes = meshState.nodes || [];
  const rttMatrix = meshState.mesh_rtt_ms || {};
  const locMap = resolveNodeLocationsJS(nodes, rttMatrix);

  const sig = JSON.stringify({
    theme: document.documentElement.dataset.theme,
    sel: selectedNode,
    scale: cfgFactoryScale,
    labels: cfgShowLabels,
    arcs: cfgShowArcs,
    nodes: nodes.map(n => {
      const g = locMap.get(n.node) || {};
      const hr = n.stats || {};
      const h = classifyNodeHealth(n);
      return `${n.node}:${(g.lat || 0).toFixed(2)}:${(g.lng || 0).toFixed(2)}:${h.severity}:${Math.round((hr.cpu_percent || 0) / 3)}:${Math.round((hr.mem_percent || 0) / 4)}`;
    })
  });
  if (!force && sig === lastVisualSignature) return locMap;
  lastVisualSignature = sig;

  clearGroup(activeH3Group);
  clearGroup(siteGroup);
  clearGroup(arcGroup);
  siteVisuals = [];
  arcVisuals = [];

  // Highlight active H3 cell for each node (Blue for focused, Yellow for warn, Red for error, faint greyscale for peers)
  for (const n of nodes) {
    const geo = locMap.get(n.node);
    if (!geo) continue;
    try {
      const centerCell = latLngToCell(geo.lat, geo.lng, 3);
      const isFocused = n.node === selectedNode;
      const health = classifyNodeHealth(n);
      const cellOutlineColor = health.severity === "error"
        ? palette.red
        : (health.severity === "warn"
          ? palette.warn
          : (isFocused ? palette.accent : palette.line));
      const hlGeom = createLandGeometry(GLOBE_RADIUS * 1.003, [centerCell]);
      if (isFocused || health.severity !== "nominal") {
        activeH3Group.add(
          new THREE.Mesh(
            hlGeom.surface,
            new THREE.MeshBasicMaterial({
              color: cellOutlineColor,
              transparent: true,
              opacity: isFocused ? 0.18 : 0.14,
              side: THREE.DoubleSide,
              depthWrite: false
            })
          )
        );
      }
      activeH3Group.add(
        new THREE.LineSegments(
          hlGeom.outlines,
          new THREE.LineBasicMaterial({
            color: cellOutlineColor,
            transparent: true,
            opacity: isFocused ? 0.95 : 0.45
          })
        )
      );
    } catch (_) {}
  }

  // Build 3D Micro-Factory Icons for each node
  for (const n of nodes) {
    const geo = locMap.get(n.node);
    if (!geo) continue;
    const isFocused = n.node === selectedNode;
    const health = classifyNodeHealth(n);
    const factoryVis = createMicroFactoryIcon(n, geo, isFocused, health);
    siteGroup.add(factoryVis.root);
    siteVisuals.push(factoryVis);
  }

  // Build Great-Circle QUIC Latency Arcs between tactical node markers (tiny pinprick ping-pong dots along arcs)
  if (cfgShowArcs && siteVisuals.length > 1) {
    const pktGeom = new THREE.SphereGeometry(0.0045, 8, 8);
    for (let i = 0; i < siteVisuals.length; i++) {
      for (let j = i + 1; j < siteVisuals.length; j++) {
        const svA = siteVisuals[i];
        const svB = siteVisuals[j];
        const touchesFocused = svA.isFocused || svB.isFocused;
        const hasErr = svA.health.severity === "error" || svB.health.severity === "error";
        const hasWarn = svA.health.severity === "warn" || svB.health.severity === "warn";

        const arcColor = hasErr
          ? palette.red
          : (hasWarn ? palette.warn : (touchesFocused ? palette.ink2 : palette.line));

        const pts = createGreatCircleArcPoints(svA.spireWorldTip, svB.spireWorldTip, 48);
        const line = new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(pts),
          new THREE.LineBasicMaterial({
            color: arcColor,
            transparent: true,
            opacity: touchesFocused ? 0.65 : 0.32
          })
        );

        const pktFwd = new THREE.Mesh(
          pktGeom,
          new THREE.MeshBasicMaterial({ color: hasErr ? palette.red : (hasWarn ? palette.warn : palette.ink2) })
        );
        const pktRev = new THREE.Mesh(
          pktGeom,
          new THREE.MeshBasicMaterial({ color: hasErr ? palette.red : (hasWarn ? palette.warn : palette.line) })
        );
        arcGroup.add(line, pktFwd, pktRev);
        const rtt = lookupPairRtt(svA.node, svB.node, nodes, rttMatrix) ?? 20.0;
        arcVisuals.push({ pts, line, pktFwd, pktRev, rtt, phase: (i + j) * 0.31 });
      }
    }
  }

  return locMap;
}

// ---------------------------------------------------------------------------
// 6b. Live Event FX: SHA-256 blob downloads (ink particle stream into the node)
//     & process starts (blue H3 cell highlight + blue particle burst).
//     Fired only from real state changes between gossip polls, or replayed from the
//     focused node's real process blob/PID during the BLB / PRC tour steps.
// ---------------------------------------------------------------------------
const fxGroup = new THREE.Group();
globeSurface.add(fxGroup);
const FX_MAX_ACTIVE = 24;
const FX_MIN_GAP_MS = 1200;
let activeFx = [];
let fxLastFiredMs = new Map(); // `${kind}:${node}` -> ms
let prevNodeFxState = null;    // node -> { blobKeys, shas, procs }
let lastTourFxKey = "";
let fxDotTexture = null;

function getFxDotTexture() {
  if (fxDotTexture) return fxDotTexture;
  const c = document.createElement("canvas");
  c.width = 64;
  c.height = 64;
  const g = c.getContext("2d");
  const grad = g.createRadialGradient(32, 32, 0, 32, 32, 32);
  grad.addColorStop(0, "rgba(255,255,255,1)");
  grad.addColorStop(0.35, "rgba(255,255,255,0.85)");
  grad.addColorStop(1, "rgba(255,255,255,0)");
  g.fillStyle = grad;
  g.fillRect(0, 0, 64, 64);
  fxDotTexture = new THREE.CanvasTexture(c);
  return fxDotTexture;
}

function fxBlending() {
  return document.documentElement.dataset.theme === "light" ? THREE.NormalBlending : THREE.AdditiveBlending;
}

function shortSha(sha) {
  return "sha256:" + String(sha || "").replace("sha256:", "").slice(0, 10);
}

// Tangent frame at a node: local +Y is the surface normal, so effects rise "up" off the globe
function createNodeFxFrame(geo) {
  const surface = latLngToSurfaceVector(geo.lat, geo.lng, GLOBE_RADIUS * 1.004);
  const normal = surface.clone().normalize();
  let east = new THREE.Vector3(0, 1, 0).cross(normal);
  if (east.lengthSq() < 1e-4) east = new THREE.Vector3(1, 0, 0);
  east.normalize();
  const south = east.clone().cross(normal).normalize();
  const frame = new THREE.Group();
  frame.position.copy(surface);
  frame.quaternion.setFromRotationMatrix(new THREE.Matrix4().makeBasis(east, normal, south));
  return frame;
}

function createFxPoints(count, color, sizePx) {
  const geom = new THREE.BufferGeometry();
  geom.setAttribute("position", new THREE.BufferAttribute(new Float32Array(count * 3), 3));
  const mat = new THREE.PointsMaterial({
    color,
    size: sizePx,
    sizeAttenuation: false,
    map: getFxDotTexture(),
    transparent: true,
    depthWrite: false,
    blending: fxBlending()
  });
  const pts = new THREE.Points(geom, mat);
  pts.frustumCulled = false;
  return pts;
}

function createFxRing(radius, segments, color) {
  const pts = [];
  for (let i = 0; i <= segments; i++) {
    const a = (i / segments) * Math.PI * 2;
    pts.push(new THREE.Vector3(Math.cos(a) * radius, 0.004, Math.sin(a) * radius));
  }
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints(pts),
    new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0, depthWrite: false })
  );
}

function createFxBeam(height, color) {
  return new THREE.Line(
    new THREE.BufferGeometry().setFromPoints([new THREE.Vector3(0, 0, 0), new THREE.Vector3(0, height, 0)]),
    new THREE.LineBasicMaterial({ color, transparent: true, opacity: 0, depthWrite: false })
  );
}

// Event tag sized to its text (createMicroTagSprite's fixed canvas clips long process names)
function createFxTag(text, colorHex) {
  const font = "700 32px 'B612 Mono', 'JetBrains Mono', ui-monospace, monospace";
  const canvas = document.createElement("canvas");
  const measure = canvas.getContext("2d");
  measure.font = font;
  canvas.width = Math.ceil(measure.measureText(text).width) + 32;
  canvas.height = 72;
  const ctx = canvas.getContext("2d");
  ctx.shadowColor = "rgba(0, 0, 0, 0.96)";
  ctx.shadowBlur = 8;
  ctx.shadowOffsetY = 2;
  ctx.fillStyle = colorHex;
  ctx.font = font;
  ctx.textBaseline = "middle";
  ctx.textAlign = "center";
  ctx.fillText(text, canvas.width / 2, 36);
  const tex = new THREE.CanvasTexture(canvas);
  tex.colorSpace = THREE.SRGBColorSpace;
  const tag = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true, opacity: 0, depthWrite: false }));
  const h = 0.11;
  tag.scale.set(h * canvas.width / canvas.height, h, 1);
  tag.renderOrder = 19;
  return tag;
}

function disposeFxObject(obj) {
  obj.traverse(o => {
    if (o.geometry) o.geometry.dispose();
    if (o.material) {
      if (o.material.map && o.material.map !== fxDotTexture) o.material.map.dispose();
      o.material.dispose();
    }
  });
}

function canFireFx(kind, node, nowMs) {
  const key = `${kind}:${node}`;
  if (nowMs - (fxLastFiredMs.get(key) || 0) < FX_MIN_GAP_MS) return false;
  fxLastFiredMs.set(key, nowMs);
  return true;
}

function pushFx(fx) {
  console.info(`[dialtone fx] ${fx.kind} @ ${fx.node}: ${fx.label}${fx.source === "tour" ? " (tour replay)" : ""}`);
  while (activeFx.length >= FX_MAX_ACTIVE) {
    const old = activeFx.shift();
    old.objects.forEach(o => { fxGroup.remove(o); disposeFxObject(o); });
  }
  fx.objects.forEach(o => fxGroup.add(o));
  activeFx.push(fx);
}

// Blob download into the node's SHA-256 store: ink particles stream down and converge into the node,
// a hexagonal "hash lock" ring closes on it, and a BLB tag names the stored digest.
function spawnBlobDownloadFx(nodeName, label, source = "live") {
  const nowMs = performance.now();
  const sv = siteVisuals.find(s => s.node === nodeName);
  if (!sv || !canFireFx("blob", nodeName, nowMs)) return;
  const inkCol = palette.accent.clone();
  const frame = createNodeFxFrame(sv.geo);

  const N = 84;
  const seeds = [];
  for (let i = 0; i < N; i++) {
    seeds.push({
      h0: 0.5 + Math.random() * 0.55,
      r0: 0.1 + Math.random() * 0.32,
      a: Math.random() * Math.PI * 2,
      delay: Math.random() * 1.1,
      fall: 0.65 + Math.random() * 0.45
    });
  }
  const stream = createFxPoints(N, inkCol, 5);
  const hexRing = createFxRing(0.45, 6, inkCol);
  const beam = createFxBeam(1.05, inkCol);
  const tag = createFxTag(`BLB ↓ ${label}`, palette.accentHex);
  // Offset toward screen-north (-south) so the tag stays readable from the overhead tour camera
  tag.position.set(0, 0.5, -0.62);
  frame.add(stream, hexRing, beam, tag);

  const dur = 3.4;
  pushFx({
    kind: "blob",
    node: nodeName,
    label: `BLB ${label}`,
    source,
    startMs: nowMs,
    dur,
    frame,
    objects: [frame],
    update(t) {
      const pos = stream.geometry.attributes.position.array;
      for (let i = 0; i < N; i++) {
        const s = seeds[i];
        const p = clamp01((t - s.delay) / s.fall);
        const e = p * p; // accelerate into the node like a payload landing
        const r = s.r0 * (1 - e);
        pos[i * 3] = Math.cos(s.a + p * 1.4) * r;
        pos[i * 3 + 1] = t < s.delay ? -10 : s.h0 * (1 - e);
        pos[i * 3 + 2] = Math.sin(s.a + p * 1.4) * r;
      }
      stream.geometry.attributes.position.needsUpdate = true;
      const fadeOut = 1 - clamp01((t - (dur - 0.7)) / 0.7);
      stream.material.opacity = 0.95 * fadeOut;
      beam.material.opacity = 0.35 * clamp01(t / 0.25) * fadeOut;
      const lockP = clamp01((t - 0.9) / 1.3);
      const ringS = 1.0 - 0.72 * easeInOutCubic(lockP);
      hexRing.scale.set(ringS, 1, ringS);
      hexRing.rotation.y = lockP * Math.PI / 3;
      hexRing.material.opacity = 0.9 * clamp01(t / 0.3) * fadeOut;
      tag.material.opacity = clamp01(t / 0.3) * fadeOut;
    }
  });
}

// Process start: the node's H3 cell lights up blue, a blue particle burst erupts from the node,
// two rings ripple outward, and a PRC tag names the process and its PID.
function spawnProcessStartFx(nodeName, procName, pid, source = "live") {
  const nowMs = performance.now();
  const sv = siteVisuals.find(s => s.node === nodeName);
  if (!sv || !canFireFx("spawn", nodeName, nowMs)) return;
  const blue = palette.blue.clone();
  const frame = createNodeFxFrame(sv.geo);
  const objects = [frame];

  // Blue highlight of the node's H3 cell plus a fainter 2-ring disk so it reads at globe zoom
  let cellFill = null;
  let cellOutline = null;
  let ringFill = null;
  let ringOutline = null;
  try {
    const center = latLngToCell(sv.geo.lat, sv.geo.lng, 3);
    const cellGeom = createLandGeometry(GLOBE_RADIUS * 1.0036, [center]);
    const ringGeom = createLandGeometry(GLOBE_RADIUS * 1.0034, gridDisk(center, 2).filter(c => c !== center));
    cellFill = new THREE.Mesh(
      cellGeom.surface,
      new THREE.MeshBasicMaterial({ color: blue, transparent: true, opacity: 0, side: THREE.DoubleSide, depthWrite: false })
    );
    cellOutline = new THREE.LineSegments(
      cellGeom.outlines,
      new THREE.LineBasicMaterial({ color: blue, transparent: true, opacity: 0, depthWrite: false })
    );
    ringFill = new THREE.Mesh(
      ringGeom.surface,
      new THREE.MeshBasicMaterial({ color: blue, transparent: true, opacity: 0, side: THREE.DoubleSide, depthWrite: false })
    );
    ringOutline = new THREE.LineSegments(
      ringGeom.outlines,
      new THREE.LineBasicMaterial({ color: blue, transparent: true, opacity: 0, depthWrite: false })
    );
    objects.push(ringFill, ringOutline, cellFill, cellOutline);
  } catch (_) {}

  const N = 110;
  const seeds = [];
  for (let i = 0; i < N; i++) {
    const a = Math.random() * Math.PI * 2;
    const spread = 0.25 + Math.random() * 0.75;
    const v = new THREE.Vector3(Math.cos(a) * spread, 0.55 + Math.random() * 1.0, Math.sin(a) * spread)
      .normalize()
      .multiplyScalar(0.45 + Math.random() * 0.75);
    seeds.push({ v, delay: Math.random() * 0.35, life: 1.4 + Math.random() * 1.1 });
  }
  const burst = createFxPoints(N, blue, 5);
  const ringA = createFxRing(0.06, 48, blue);
  const ringB = createFxRing(0.06, 48, blue);
  const beam = createFxBeam(0.85, blue);
  const tag = createFxTag(`PRC ▶ ${procName} PID ${pid}`, palette.blueHex);
  tag.position.set(0, 0.4, -0.62);
  frame.add(burst, ringA, ringB, beam, tag);

  const dur = 4.2;
  const drag = 2.2;
  pushFx({
    kind: "spawn",
    node: nodeName,
    label: `${procName} PID ${pid}`,
    source,
    startMs: nowMs,
    dur,
    frame,
    objects,
    update(t) {
      const pos = burst.geometry.attributes.position.array;
      for (let i = 0; i < N; i++) {
        const s = seeds[i];
        const lt = t - s.delay;
        if (lt < 0 || lt > s.life) {
          pos[i * 3 + 1] = -10;
          continue;
        }
        const travel = (1 - Math.exp(-drag * lt)) / drag; // decelerating sparks
        pos[i * 3] = s.v.x * travel;
        pos[i * 3 + 1] = s.v.y * travel;
        pos[i * 3 + 2] = s.v.z * travel;
      }
      burst.geometry.attributes.position.needsUpdate = true;
      const fadeOut = 1 - clamp01((t - (dur - 0.9)) / 0.9);
      burst.material.opacity = fadeOut;
      [[ringA, 0.0], [ringB, 0.38]].forEach(([ring, off]) => {
        const p = clamp01((t - off) / 1.5);
        const s = 1 + p * 9;
        ring.scale.set(s, 1, s);
        ring.material.opacity = t < off ? 0 : 0.9 * (1 - p);
      });
      beam.material.opacity = 0.8 * (1 - clamp01(t / 1.2));
      tag.material.opacity = clamp01(t / 0.3) * fadeOut;
      if (cellFill) {
        const pulse = 0.65 + 0.35 * Math.cos(t * 6.0);
        const fadeIn = clamp01(t / 0.2);
        cellFill.material.opacity = 0.55 * pulse * fadeOut * fadeIn;
        cellOutline.material.opacity = 0.95 * fadeOut * fadeIn;
        // Surrounding disk lights up as the ripple passes over it
        const ringIn = clamp01((t - 0.15) / 0.4) * fadeOut;
        ringFill.material.opacity = 0.28 * ringIn;
        ringOutline.material.opacity = 0.7 * ringIn;
      }
    }
  });
}

function updateLiveFx(nowMs, fxScale) {
  activeFx = activeFx.filter(fx => {
    const t = (nowMs - fx.startMs) / 1000;
    if (t > fx.dur) {
      fx.objects.forEach(o => { fxGroup.remove(o); disposeFxObject(o); });
      return false;
    }
    fx.frame.scale.setScalar(fxScale);
    fx.update(t);
    return true;
  });
}

function snapshotNodeFxState(n) {
  const procs = new Map();
  const shas = new Set();
  for (const p of (n.processes || n.processes || [])) {
    if (!p || !p.name) continue;
    if (p.digest) shas.add(p.digest);
    procs.set(p.name, { pid: p.pid || null, status: p.status || "" });
  }
  const probe = n.probe || {};
  if (probe.program_digest) shas.add(probe.program_digest);
  const bs = n.store || {};
  return {
    blobKeys: Number.isFinite(bs.blob_keys) ? bs.blob_keys : null,
    latestSha: bs.latest_digest || "",
    shas,
    procs
  };
}

// Diff consecutive gossip snapshots per node and fire FX for real blob downloads & process starts
function detectNodeFxEvents(nodes, fire = true) {
  const next = new Map();
  for (const n of nodes) next.set(n.node, snapshotNodeFxState(n));
  if (prevNodeFxState && fire) {
    for (const n of nodes) {
      const cur = next.get(n.node);
      const prev = prevNodeFxState.get(n.node);
      if (!prev) continue; // first sighting of this node: nothing to diff against yet
      const newShas = [...cur.shas].filter(s => !prev.shas.has(s));
      const keyDelta = cur.blobKeys != null && prev.blobKeys != null ? cur.blobKeys - prev.blobKeys : 0;
      const latestChanged = cur.latestSha && cur.latestSha !== prev.latestSha;
      if (newShas.length > 0 || keyDelta > 0) {
        const label = newShas.length > 0
          ? shortSha(newShas[0])
          : (latestChanged ? shortSha(cur.latestSha) : `+${keyDelta} BLOB${keyDelta === 1 ? "" : "S"}`);
        spawnBlobDownloadFx(n.node, label);
      }
      for (const [name, p] of cur.procs) {
        const before = prev.procs.get(name);
        if (p.status === "running" && p.pid && (!before || before.pid !== p.pid)) {
          spawnProcessStartFx(n.node, name, p.pid);
        }
      }
    }
  }
  prevNodeFxState = next;
}

// BLB / PRC tour steps replay the focused node's real process blob digest and running PID
function replayTourStepFx(stepIdx, nodeName) {
  const n = (meshState.nodes || []).find(x => x.node === nodeName);
  if (!n) return;
  const procs = n.processes || n.processes || [];
  const probe = n.probe || {};
  const running = procs.find(p => p && p.status === "running" && p.pid) || null;
  if (stepIdx === 1) {
    const sha = probe.program_digest || (running && running.digest) || "";
    if (sha) spawnBlobDownloadFx(nodeName, shortSha(sha), "tour");
  } else if (stepIdx === 2 && running) {
    spawnProcessStartFx(nodeName, running.name, running.pid, "tour");
  }
}

// ---------------------------------------------------------------------------
// 7. Overhead Geospatial Camera & Endless Tour Loop
// ---------------------------------------------------------------------------
const raycaster = new THREE.Raycaster();
const pointerNDC = new THREE.Vector2();

let tourFromQuat = new THREE.Quaternion();
let tourToQuat = new THREE.Quaternion();
let tourFromDist = 4.5;
let tourFlightStartMs = performance.now();
let lastTourSwitchMs = performance.now();
let tourInitialized = false;

function easeInOutCubic(t) {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

// North-Up overhead (nadir) geospatial orientation:
// Places the node directly beneath the camera (+Z) with local North pointing straight up (+Y)
function targetQuatForGeo(geo) {
  const siteNormal = latLngToSurfaceVector(geo.lat, geo.lng, 1).normalize();
  let east = new THREE.Vector3(0, 1, 0).cross(siteNormal);
  if (east.lengthSq() < 1e-4) east = new THREE.Vector3(1, 0, 0);
  east.normalize();
  const north = siteNormal.clone().cross(east).normalize();
  const basis = new THREE.Matrix4().makeBasis(east, north, siteNormal).invert();
  return new THREE.Quaternion().setFromRotationMatrix(basis);
}

function setUserZoomLayer(newLayer) {
  const clamped = Math.max(1, Math.min(5, Math.round(Number(newLayer) || 1)));
  currentZoomLayer = clamped;
  layerMode = clamped;
  const layerSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[0];
  targetCameraDist = layerSpec.camDist;

  const zSlider = document.getElementById("slider-zoom-layer");
  const zVal = document.getElementById("val-zoom-layer");
  if (zSlider && zVal) {
    zSlider.value = String(currentZoomLayer);
    zVal.textContent = `${currentZoomLayer}.0 (${layerSpec.name})`;
  }
  renderTopShellStats();
  syncMenuControls();
}

const TOUR_STEPS = [
  { idx: 0, code: "MSH", ring: 1, rowKey: "MSH", title: "ENCRYPTED GOSSIP MESH" },
    { idx: 1, code: "BLB", ring: 2, rowKey: "PRB", title: "BLOB STORE & PEER PULLS" },
  { idx: 2, code: "PRC", ring: 3, rowKey: "PRB", title: "PROCESS LOAD & EXEC FROM BLOB" },
  { idx: 3, code: "NET", ring: 1, rowKey: "NET", title: "BEATS & RTT" },
  { idx: 4, code: "DEV", ring: 3, rowKey: "DEV", title: "CONNECTED DEVICES" }
];

// Devices connected to the focused node (from its details), as short codes for the overlay.
const DEVICE_CODES = {
  camera: "CAM", microphone: "MIC", speaker: "SPK", keyboard: "KBD", pointer: "PTR", display: "DSP",
  printer: "PRN", storage: "STO", bluetooth: "BT", gamepad: "PAD", other: "DEV"
};
const DEVICE_LIST_ROWS = 8;
const DEVICE_LIST_COLS = 2;

// The focused node's devices: a header line, then two devices per line.
function renderDeviceLines(target, isFocusStep) {
  const devices = Array.isArray(target.devices) ? target.devices : [];
  const codeCls = isFocusStep ? "focus" : "dim";
  const lines = [
    `${isFocusStep ? `<span class="focus">▸</span> ` : "  "}<span class="${codeCls}">DEVICES</span> <span class="hi">${target.node}</span> <span class="dim">(${devices.length} connected)</span>`
  ];
  if (devices.length === 0) {
    lines.push(`<span class="dim">  no devices reported yet</span>`);
    return lines;
  }
    const cell = (d) => {
    const label = d.detail ? `${d.name} (${d.detail})` : d.name;
    return `<span class="${codeCls}">${padR(DEVICE_CODES[d.kind] || "DEV", 4)}</span><span class="hi">${padR(label, 44)}</span> <span class="dim">${padR(d.bus || "", 10)}</span>`;
  };
  const shown = devices.slice(0, DEVICE_LIST_ROWS * DEVICE_LIST_COLS);
  for (let i = 0; i < shown.length; i += DEVICE_LIST_COLS) {
    lines.push("  " + shown.slice(i, i + DEVICE_LIST_COLS).map(cell).join("  "));
  }
  if (devices.length > shown.length) {
    lines.push(`<span class="dim">  +${devices.length - shown.length} more</span>`);
  }
  return lines;
}
let activeTourStep = 0;
let tourStepChangedNode = true;

function buildTourSequence() {
  const nodes = meshState.nodes || [];
  if (nodes.length === 0) return [];
  // Whole-globe system tour: visits each node and steps through the 5 probe checks, the
  // network and the devices connected to it
  // while keeping the camera zoomed out on the full globe.
  const seq = [];
  for (const n of nodes) {
    for (const st of TOUR_STEPS) {
      seq.push({ node: n.node, stepIndex: st.idx });
    }
  }
  return seq;
}

function applyStop(stop, nowMs = performance.now()) {
  if (!stop) return;
  const prevNode = selectedNode;
  selectedNode = stop.node;
  activeTourStep = typeof stop.stepIndex === "number" ? (stop.stepIndex % TOUR_STEPS.length) : 0;
  const stepMeta = TOUR_STEPS[activeTourStep] || TOUR_STEPS[0];
  activeScanRing = stepMeta.ring;
  tourStepChangedNode = (prevNode !== selectedNode) || !tourInitialized;

  if (typeof stop.layer === "number") {
    setUserZoomLayer(stop.layer);
  }

  if (tourStepChangedNode) {
    userDragOffset.identity();
  }
  rebuildGlobeNodesAndArcs(tourStepChangedNode);
  renderTopShellStats();
  syncMenuControls();

  const targetVisual = siteVisuals.find(sv => sv.node === selectedNode) || siteVisuals[0];
  if (targetVisual && tourStepChangedNode) {
    const destQuat = targetQuatForGeo(targetVisual.geo);
    if (!tourInitialized) {
      globeSurface.quaternion.copy(destQuat);
      tourFromQuat.copy(destQuat);
      tourToQuat.copy(destQuat);
      currentCameraDist = targetCameraDist;
      tourFromDist = targetCameraDist;
      tourInitialized = true;
    } else {
      tourFromQuat.copy(globeSurface.quaternion);
      tourToQuat.copy(destQuat);
      tourFromDist = currentCameraDist;
      tourFlightStartMs = nowMs;
    }
  }
  lastTourSwitchMs = nowMs;
}

function stepTour(deltaSteps, nowMs = performance.now()) {
  tourStops = buildTourSequence();
  if (tourStops.length === 0) return;
  tourStopIndex = (tourStopIndex + deltaSteps + tourStops.length) % tourStops.length;
  applyStop(tourStops[tourStopIndex], nowMs);
}

function selectTourStepManual(stepIdx, nowMs = performance.now()) {
  tourStops = buildTourSequence();
  const clampedStep = ((stepIdx % TOUR_STEPS.length) + TOUR_STEPS.length) % TOUR_STEPS.length;
  const matchIdx = tourStops.findIndex(s => s.node === selectedNode && s.stepIndex === clampedStep);
  if (matchIdx >= 0) {
    tourStopIndex = matchIdx;
    applyStop(tourStops[tourStopIndex], nowMs);
  } else {
    applyStop({ node: selectedNode, stepIndex: clampedStep }, nowMs);
  }
}

function focusNodeManual(nodeName, layerOverride = null) {
  const nowMs = performance.now();
  tourStops = buildTourSequence();
  const matchIdx = tourStops.findIndex(s => s.node === nodeName && s.stepIndex === activeTourStep);
  if (matchIdx >= 0) {
    tourStopIndex = matchIdx;
  }
  const stop = { node: nodeName, stepIndex: activeTourStep };
  if (typeof layerOverride === "number") {
    stop.layer = layerOverride;
  }
  applyStop(stop, nowMs);
}

// ---------------------------------------------------------------------------
// 8. Aligned 'top' Grid Overlay (Overall Bar Graphs + Per-Row Node Stats Only)
// ---------------------------------------------------------------------------
function asciiBar(pct, warnAt, errAt, slots = 16) {
  const clamped = Math.max(0, Math.min(100, Number(pct) || 0));
  const filled = Math.max(0, Math.min(slots, Math.round((clamped / 100) * slots)));
  const empty = slots - filled;
  const cls = clamped >= errAt ? "fill err" : (clamped >= warnAt ? "fill warn" : "fill");
  return `<span class="ascii-bar">[<span class="${cls}">${"|".repeat(filled)}</span>${".".repeat(empty)}]</span>`;
}

function fmtGiB(bytes) {
  return (bytes / (1024 * 1024 * 1024)).toFixed(1);
}

function fmtBytesShort(bytes) {
  const b = Number(bytes) || 0;
  if (b >= 1024 * 1024 * 1024) return (b / (1024 * 1024 * 1024)).toFixed(1) + "G";
  if (b >= 1024 * 1024) return (b / (1024 * 1024)).toFixed(1) + "M";
  if (b >= 1024) return (b / 1024).toFixed(0) + "K";
  return b + "B";
}

function padR(str, len) {
  const s = String(str);
  return s.length >= len ? s.slice(0, len) : s + " ".repeat(len - s.length);
}

function padL(str, len) {
  const s = String(str);
  return s.length >= len ? s.slice(0, len) : " ".repeat(len - s.length) + s;
}

function valCls(val, warnAt, errAt) {
  return val >= errAt ? "err" : (val >= warnAt ? "warn" : "hi");
}

function renderTopShellStats() {
  const nodes = meshState.nodes || [];
  rebuildGlobeNodesAndArcs(false);
  if (!nodes.length) return;

  const idx = Math.max(0, nodes.findIndex(n => n.node === selectedNode));
  const target = withDetails(nodes[idx] || nodes[0]);
  const health = classifyNodeHealth(target);

  const termEl = document.getElementById("terminal-overlay");
  if (termEl) termEl.dataset.severity = health.severity;

  // Mesh configuration, channel & store stats
  const mcfg = meshState.mesh_config || {};
  const bstats = meshState.blob_stats || {};
  const channelName = mcfg.channel || "dialtone";
  const channelFp = mcfg.channel_fingerprint || "";
  const isConfigured = Boolean(mcfg.configured ?? meshState.channel_configured ?? true);
  const expectedPeers = Math.max(nodes.length, (mcfg.allowlist || []).length || nodes.length);
  const expectedLinks = expectedPeers > 1 ? (expectedPeers * (expectedPeers - 1)) / 2 : 1;
  const activeLinks = Object.keys(meshState.mesh_rtt_ms || {}).length;
  const blobKeys = bstats.blob_keys ?? 0;
  const blobBytes = bstats.blob_bytes ?? 0;
  

  // Update channel name & mesh status line in the bottom terminal overlay
  const chLineEl = document.getElementById("channel-line");
  if (chLineEl) {
    const meshUp = isConfigured && nodes.length >= 2 && activeLinks >= 1;
    const statusText = meshUp ? "ACTIVE" : (isConfigured ? "DEGRADED" : "STANDBY");
    const statusCls = meshUp ? "focus" : "warn";
    const fpPart = channelFp ? ` <span class="dim">[${channelFp}]</span>` : "";
    chLineEl.innerHTML =`<span class="dim">CH:</span> <span class="focus">${channelName}</span>${fpPart}  <span class="${statusCls}">${statusText} ${nodes.length}/${expectedPeers}</span>`;
  }

  // 1. Compute Overall Mesh / Network / probe / Focused Node Stats for the 6 Aligned Bar-Graph Rows
  let totalCores = 0;
  let totalJobs = 0;
  let sumCpu = 0;
  let sumMemUsed = 0;
  let sumMemTot = 0;
  let sumDskUsed = 0;
  let sumDskTot = 0;
  let sumRxKb = 0;
  let sumTxKb = 0;
  let totalHbs = 0;
  let directPaths = 0;
  let relayPaths = 0;
  // The probe's checks (probe.rs), counted over every node that has reported one: MSH, NBR
  // and BEAT every 30s; STO, PUL and PRC after a full probe (`dialtone probe --all`).
  let probePassTotal = 0;
  let probeCheckTotal = 0;
  const capOk = { MSH: 0, NBR: 0, BEAT: 0, STO: 0, PUL: 0, PRC: 0 };
  // How many nodes reported each check: STO, PUL and PRC only after a full probe.
  const capRan = { MSH: 0, NBR: 0, BEAT: 0, STO: 0, PUL: 0, PRC: 0 };
  const pullMs = [];
  const allRtts = [];
  const allJitters = [];

  for (const n of nodes) {
    const h = n.stats || {};
    const procs = n.processes || n.processes || [];
    const runningProcs = procs.filter(p => p && p.status === "running");
    totalCores += (h.cpu_cores || 8);
    totalJobs += runningProcs.length;
    totalHbs += (n.counter || 0);
    sumCpu += (h.cpu_percent || 0);
    sumMemUsed += (h.mem_used_bytes || 0);
    sumMemTot += (h.mem_total_bytes || 1);
    sumDskUsed += (h.disk_used_bytes || 0);
    sumDskTot += (h.disk_total_bytes || 1);
    const nic0 = (h.interfaces && h.interfaces[0]) || { rx_bytes_per_sec: 0, tx_bytes_per_sec: 0 };
    sumRxKb += (nic0.rx_bytes_per_sec || 0) / 1024;
    sumTxKb += (nic0.tx_bytes_per_sec || 0) / 1024;
    for (const ms of Object.values(n.peer_rtt_ms || {})) {
      const v = Number(ms);
      if (Number.isFinite(v) && v > 0) allRtts.push(v);
    }
    const nss = n.summary_stats || null;
    if (nss && nss.rtt_ms && typeof nss.rtt_ms.stddev === "number") {
      allJitters.push(nss.rtt_ms.stddev);
    }
    const obs = n.observed_remote_addrs || [];
    if (obs.some(a => String(a).startsWith("active:ip:") || String(a).startsWith("local:self"))) {
      directPaths += 1;
    } else if (obs.some(a => String(a).includes("relay"))) {
      relayPaths += 1;
    } else {
      directPaths += 1;
    }

        const checks = (n.probe && Array.isArray(n.probe.checks)) ? n.probe.checks : [];
    for (const c of checks) {
      probeCheckTotal += 1;
      if (c.code in capRan) capRan[c.code] += 1;
      if (c.ok) {
        probePassTotal += 1;
        if (c.code in capOk) capOk[c.code] += 1;
      }
    }
    for (const p of (n.probe && n.probe.pulls) || []) {
      if (p.ok && Number.isFinite(p.ms)) pullMs.push(p.ms);
    }
  }

  const hr = target.stats || {};
  const ss = target.summary_stats || null;
  const cpuVal = hr.cpu_percent || 0;
  const memVal = hr.mem_percent || 0;
  const dskVal = hr.disk_percent || 0;

  const cpuStd = (ss && ss.cpu_pct && typeof ss.cpu_pct.stddev === "number") ? ss.cpu_pct.stddev : 0;
  const cpuAvg = (ss && ss.cpu_pct && typeof ss.cpu_pct.mean === "number") ? ss.cpu_pct.mean : (sumCpu / nodes.length);
  const cpuMax = (ss && ss.cpu_pct && typeof ss.cpu_pct.max === "number") ? ss.cpu_pct.max : cpuVal;
  const memStd = (ss && ss.mem_pct && typeof ss.mem_pct.stddev === "number") ? ss.mem_pct.stddev : 0;
  const thrashScore = (ss && typeof ss.thrashing_score === "number") ? ss.thrashing_score : 0;

  const avgRtt = allRtts.length ? (allRtts.reduce((a, b) => a + b, 0) / allRtts.length) : 0;
  const minRtt = allRtts.length ? Math.min(...allRtts) : 0;
  const maxRtt = allRtts.length ? Math.max(...allRtts) : 0;
  const avgJitter = allJitters.length ? (allJitters.reduce((a, b) => a + b, 0) / allJitters.length) : 0;
  const rttBarPct = Math.min(100, (avgRtt / 250) * 100);
  const blobFetches = bstats.blob_fetches ?? 0;

  const peerRatio = Math.min(1, nodes.length / Math.max(1, expectedPeers));
  const linkRatio = Math.min(1, activeLinks / Math.max(1, expectedLinks));
  const meshHealthPct = Math.round(((peerRatio * 0.5) + (linkRatio * 0.5)) * 100);
  const mshCls = meshHealthPct >= 90 ? "hi" : (meshHealthPct >= 50 ? "warn" : "err");

  const probePct = probeCheckTotal > 0 ? Math.round((probePassTotal / probeCheckTotal) * 100) : 100;
  const probeCls = probePct >= 100 ? "hi" : (probePct >= 60 ? "warn" : "err");
  
  const targetCycle = (target.probe && target.probe.cycle) ? target.probe.cycle : 1;

  const stepMeta = TOUR_STEPS[activeTourStep] || TOUR_STEPS[0];
  const rowMark = (key) => stepMeta.rowKey === key ? `<span class="focus">▸</span> ` : `  `;
  const rowLbl = (key, label) => `<span class="${stepMeta.rowKey === key ? "focus" : "dim"}">${padR(label, 4)}</span>`;
  const kvCell = (k, v, cls = "hi", isStepFocus = false) => {
    const kCls = isStepFocus ? "focus" : "dim";
    const vCls = isStepFocus ? "focus" : cls;
    return `<span class="${kCls}">${padR(k, 4)}</span><span class="${vCls}">${padL(v, 8)}</span>`;
  };
  // A check no node has run lately (a full-probe check) shows "-", not a failure.
  const capCell = (code, step) => capRan[code]
    ? kvCell(code, `${capOk[code]}/${capRan[code]} OK`, capOk[code] >= capRan[code] ? "hi" : (capOk[code] > 0 ? "warn" : "err"), stepMeta.code === step)
    : kvCell(code, "-", "dim", stepMeta.code === step);

  // Overall Stats Grid (6 strictly aligned bar-graph rows: MSH, NET, probe, CPU, MEM, DSK)
  const overallRows = [
    `${rowMark("MSH")}${rowLbl("MSH", "MSH")} ${asciiBar(meshHealthPct, 101, 101, 18)} <span class="${mshCls}">${padL(meshHealthPct.toFixed(0) + "%", 7)}</span>   ${kvCell("NDS", `${nodes.length}/${expectedPeers}`, "hi", stepMeta.code === "MSH")}   ${kvCell("LNK", `${activeLinks}/${expectedLinks}`, "hi", stepMeta.code === "MSH")}   ${kvCell("PTH", `${directPaths}D/${relayPaths}R`)}   ${kvCell("BTS", "#" + totalHbs)}   ${kvCell("DIG", String(blobKeys), "hi", stepMeta.code === "BLB")}`,
    `${rowMark("NET")}${rowLbl("NET", "NET")} ${asciiBar(rttBarPct, 75, 95, 18)} <span class="${valCls(avgRtt, 180, 260)}">${padL(avgRtt.toFixed(1) + "ms", 7)}</span>   ${kvCell("RX", sumRxKb.toFixed(0) + "K/s", "hi", stepMeta.code === "NET")}   ${kvCell("TX", sumTxKb.toFixed(0) + "K/s", "hi", stepMeta.code === "NET")}   ${kvCell("MIN", minRtt.toFixed(1) + "ms")}   ${kvCell("MAX", maxRtt.toFixed(1) + "ms")}   ${kvCell("JIT", "±" + avgJitter.toFixed(1) + "ms")}`,
    `${rowMark("PRB")}${rowLbl("PRB", "PRB")} ${asciiBar(probePct, 101, 101, 18)} <span class="${probeCls}">${padL(`${probePassTotal}/${probeCheckTotal}`, 7)}</span>   ${capCell("MSH", "MSH")}   ${capCell("NBR", "NET")}   ${capCell("BEAT", "NET")}   ${capCell("PUL", "BLB")}   ${capCell("PRC", "PRC")}`,
    `${rowMark("CPU")}${rowLbl("CPU", "CPU")} ${asciiBar(cpuVal, WARN_CPU_PCT, ERROR_CPU_PCT, 18)} <span class="${valCls(cpuVal, WARN_CPU_PCT, ERROR_CPU_PCT)}">${padL(cpuVal.toFixed(1) + "%", 7)}</span>   ${kvCell("AVG", cpuAvg.toFixed(1) + "%")}   ${kvCell("STD", "±" + cpuStd.toFixed(1) + "%")}   ${kvCell("MAX", cpuMax.toFixed(1) + "%")}   ${kvCell("COR", String(totalCores))}   ${kvCell("STO", fmtBytesShort(blobBytes), "hi", stepMeta.code === "BLB")}`,
    `${rowMark("MEM")}${rowLbl("MEM", "MEM")} ${asciiBar(memVal, WARN_MEM_PCT, ERROR_MEM_PCT, 18)} <span class="${valCls(memVal, WARN_MEM_PCT, ERROR_MEM_PCT)}">${padL(memVal.toFixed(1) + "%", 7)}</span>   ${kvCell("USE", fmtGiB(sumMemUsed) + "G")}   ${kvCell("TOT", fmtGiB(sumMemTot) + "G")}   ${kvCell("STD", "±" + memStd.toFixed(1) + "%")}   ${kvCell("PUL", (pullMs.length ? (pullMs.reduce((a, b) => a + b, 0) / pullMs.length).toFixed(0) : "-") + "ms", "hi", stepMeta.code === "BLB")}   ${kvCell("FET", String(blobFetches))}`,
    `${rowMark("DSK")}${rowLbl("DSK", "DSK")} ${asciiBar(dskVal, WARN_DISK_PCT, ERROR_DISK_PCT, 18)} <span class="${valCls(dskVal, WARN_DISK_PCT, ERROR_DISK_PCT)}">${padL(dskVal.toFixed(1) + "%", 7)}</span>   ${kvCell("USE", fmtGiB(sumDskUsed) + "G")}   ${kvCell("TOT", fmtGiB(sumDskTot) + "G")}   ${kvCell("THR", thrashScore.toFixed(0), thrashScore > 0 ? "err" : "hi")}   ${kvCell("PRB", `#${targetCycle}`, "hi")}   ${kvCell("PCS", `${totalJobs}/${nodes.length}`, "hi", stepMeta.code === "PRC")}`
  ];

  // 2. Per-Row Stats Grid (Strictly aligned table with inline CPU / MEM / DSK bar graphs + network/link/blob-proc/probe stats per node)
  const headerLine =
    `<span class="dim">  ${padR("NODE", 9)} ${padR("ARCH", 10)} ${padR("CPU", 17)} ${padR("MEM", 17)} ${padR("DSK", 17)} ${padR("IP/PATH", 16)} ${padL("RX/TX", 9)} ${padL("LNK", 4)} ${padL("RTT", 7)} ${padL("JIT", 6)} ${padL("PROC", 10)} ${padL("PRB", 9)} ${padL("BEAT", 6)}</span>`;

  const nodeRows = nodes.map((n) => {
    const isSel = n.node === target.node;
    const nhr = n.stats || {};
    const nss = n.summary_stats || null;
    const nCpu = nhr.cpu_percent || 0;
    const nMem = nhr.mem_percent || 0;
    const nDsk = nhr.disk_percent || 0;

    const rttVals = Object.values(n.peer_rtt_ms || {}).map(Number).filter(v => Number.isFinite(v) && v > 0);
    const nRtt = rttVals.length ? (rttVals.reduce((a, b) => a + b, 0) / rttVals.length) : 0;
    const rttStr = nRtt > 0 ? `${nRtt.toFixed(1)}ms` : " 0.0ms";
    const nJit = (nss && nss.rtt_ms && typeof nss.rtt_ms.stddev === "number") ? nss.rtt_ms.stddev : 0;
    const jitStr = `±${nJit.toFixed(1)}`;
    const peerLinks = `${rttVals.length}/${Math.max(1, nodes.length - 1)}`;

    const nic0 = (nhr.interfaces && nhr.interfaces[0]) || { rx_bytes_per_sec: 0, tx_bytes_per_sec: 0 };
    const rxK = Math.round((nic0.rx_bytes_per_sec || 0) / 1024);
    const txK = Math.round((nic0.tx_bytes_per_sec || 0) / 1024);
    const rxTxStr = `${rxK}/${txK}K`;

    const tr = n.iroh_transport || {};
    const rawIp = tr.bound_non_tailnet_ipv4 || (tr.advertised_ip_addrs && tr.advertised_ip_addrs[0]) || "0.0.0.0";
    const cleanIp = String(rawIp).split(":")[0];
    const obs = n.observed_remote_addrs || [];
    const pathTag = obs.some(a => String(a).startsWith("active:relay:")) && !obs.some(a => String(a).startsWith("active:ip:")) ? "R" : "D";
    const ipPathCol = padR(`${cleanIp}/${pathTag}`, 16);

    const shortOs = (n.os || "os").replace("macos", "mac").replace("linux", "lin").replace("windows", "win");
    const shortArch = (n.arch || "arch").replace("aarch64", "arm64");
    const archCol = padR(`${shortOs}/${shortArch}`, 10);

    const nProcs = n.processes || n.processes || [];
    const activeProc = nProcs.find(p => p && p.status === "running") || nProcs[0] || null;
    let procStr = "NONE";
    let procCls = "dim";
    if (activeProc) {
      const sha4 = String(activeProc.digest || "").replace("sha256:", "").slice(0, 4);
      if (activeProc.status === "running") {
        procStr = activeProc.pid ? `${sha4}:${activeProc.pid}` : `${sha4}:RUN`;
        procCls = "hi";
      } else {
        procStr = `${sha4}:STOP`;
        procCls = "warn";
      }
    }

    const nprobe = n.probe || null;
        const probePass = nprobe ? (nprobe.passed_count ?? 0) : 0;
    const probeTotal = nprobe ? (nprobe.total_count ?? 5) : 5;
    const probeCyc = nprobe && nprobe.cycle ? nprobe.cycle : 1;
        const probeStr = nprobe ? `${probePass}/${probeTotal} #${probeCyc}` : "-";
    const probeCellCls = !nprobe ? "dim" : (probePass >= probeTotal ? "hi" : (probePass >= probeTotal - 1 ? "warn" : "err"));

    const mark = isSel ? `<span class="focus">▸</span> ` : `  `;
    const nameCls = isSel ? "focus" : "hi";
    const cpuCell = `${asciiBar(nCpu, WARN_CPU_PCT, ERROR_CPU_PCT, 8)} <span class="${valCls(nCpu, WARN_CPU_PCT, ERROR_CPU_PCT)}">${padL(nCpu.toFixed(1) + "%", 6)}</span>`;
    const memCell = `${asciiBar(nMem, WARN_MEM_PCT, ERROR_MEM_PCT, 8)} <span class="${valCls(nMem, WARN_MEM_PCT, ERROR_MEM_PCT)}">${padL(nMem.toFixed(1) + "%", 6)}</span>`;
    const dskCell = `${asciiBar(nDsk, WARN_DISK_PCT, ERROR_DISK_PCT, 8)} <span class="${valCls(nDsk, WARN_DISK_PCT, ERROR_DISK_PCT)}">${padL(nDsk.toFixed(1) + "%", 6)}</span>`;
    const ipCell = `<span class="dim">${ipPathCol}</span>`;
    const netCell = `<span class="hi">${padL(rxTxStr, 9)}</span>`;
    const lnkCell = `<span class="${rttVals.length >= Math.max(1, nodes.length - 1) ? "hi" : "warn"}">${padL(peerLinks, 4)}</span>`;
    const rttCell = `<span class="${valCls(nRtt, 180, 260)}">${padL(rttStr, 7)}</span>`;
    const jitCell = `<span class="dim">${padL(jitStr, 6)}</span>`;
    const procCell = `<span class="${procCls}">${padL(procStr, 10)}</span>`;
    const probeCell = `<span class="${probeCellCls}">${padL(probeStr, 9)}</span>`;
    const hbCell = `<span class="dim">${padL("#" + (n.counter || 0), 6)}</span>`;

    return `<span class="top-row" data-node="${n.node}">${mark}<span class="${nameCls}">${padR(n.node, 9)}</span> <span class="dim">${archCol}</span> ${cpuCell} ${memCell} ${dskCell} ${ipCell} ${netCell} ${lnkCell} ${rttCell} ${jitCell} ${procCell} ${probeCell} ${hbCell}</span>`;
  });

  const hudEl = document.getElementById("node-top-hud");
    const deviceLines = renderDeviceLines(target, stepMeta.code === "DEV");
  hudEl.innerHTML = [...overallRows, "", headerLine, ...nodeRows, "", ...deviceLines].join("\n");

  renderTourClosedCaptions({
    nodes,
    target,
    stepMeta,
    channelName,
    channelFp,
    expectedPeers,
    activeLinks,
    expectedLinks,
    blobKeys,
    blobBytes,
    avgRtt,
    avgJitter,
    sumRxKb,
    sumTxKb
  });
}

function renderTourClosedCaptions(ctx) {
  const ccHeaderEl = document.getElementById("tour-cc-header");
  const ccBodyEl = document.getElementById("tour-cc-body");
  if (!ccHeaderEl || !ccBodyEl) return;

  const {
    nodes,
    target,
    stepMeta,
    channelName,
    channelFp,
    expectedPeers,
    activeLinks,
    expectedLinks,
    blobKeys,
    blobBytes,
    avgRtt,
    avgJitter
  } = ctx;

  const tprobe = target.probe || {};
  const tProcs = target.processes || target.processes || [];
  const activeProc = tProcs.find(p => p && p.status === "running") || tProcs[0] || {};
  const programName = tprobe.program_name || activeProc.name || "probe";
  const programDigest = tprobe.program_digest || activeProc.digest || "sha256:181091cbf978";
  const shortSha = String(programDigest).replace("sha256:", "").slice(0, 12);
  const processPid = tprobe.pid || activeProc.pid || "LIVE";
  const stagedPath = tprobe.staged_path || activeProc.path || `~/.dialtone/run/${programName}-${shortSha}`;
    const probeCheck = (code) => (Array.isArray(tprobe.checks) ? tprobe.checks : []).find(c => c.code === code) || null;
  const checkMark = (c) => !c ? `<span class="dim">pending</span>` : (c.ok ? `<span class="hi">OK</span>` : `<span class="err">FAIL</span>`);
  const okMark = (ok) => ok ? `<span class="hi">✓</span>` : `<span class="err">✗</span>`;

  const tr = target.iroh_transport || {};
  const rawIp = tr.bound_non_tailnet_ipv4 || (tr.advertised_ip_addrs && tr.advertised_ip_addrs[0]) || "0.0.0.0";
  const cleanIp = String(rawIp).split(":")[0];
  const obs = target.observed_remote_addrs || [];
  const pathMode = obs.some(a => String(a).startsWith("active:relay:")) && !obs.some(a => String(a).startsWith("active:ip:"))
    ? "RELAY"
    : "DIRECT QUIC";

  const peerRttEntries = Object.entries(target.peer_rtt_ms || {})
    .map(([peer, ms]) => `${peer}=${Number(ms).toFixed(1)}ms`)
    .join(", ") || "timing neighbor Rtts...";

    const cycleNum = tprobe.cycle || 0;
  const passCnt = tprobe.passed_count ?? 0;
  const totCnt = tprobe.total_count ?? 3;

  const stepPills = TOUR_STEPS.map(s => {
    const isAct = s.idx === stepMeta.idx;
    return `<button type="button" class="cc-step-btn${isAct ? " is-active" : ""}" data-tour-step="${s.idx}">[${s.idx + 1} ${s.code}]</button>`;
  }).join("");

  const playGlyph = tourPlaying ? "⏸" : "▶";
  ccHeaderEl.innerHTML =
    `<div><span class="cc-badge">[CC]</span><span class="focus">STEP ${stepMeta.idx + 1}/${TOUR_STEPS.length} • ${stepMeta.title}</span>  <span class="dim">•  NODE:</span> <span class="hi">${target.node}</span> <span class="dim">(${target.os || "os"}/${target.arch || "arch"})</span></div>` +
    `<div class="cc-steps">${stepPills}<button type="button" class="cc-step-btn" data-tour-nav="-1" title="Previous Step">◀</button><button type="button" class="cc-step-btn" data-tour-toggle="1" title="Pause/Resume Tour">${playGlyph}</button><button type="button" class="cc-step-btn" data-tour-nav="1" title="Next Step">▶</button></div>`;

  let line1 = "";
  let line2 = "";
  switch (stepMeta.idx) {
    case 0: // MSH: Encrypted Gossip Mesh
      line1 = `<span class="focus">[1/${TOUR_STEPS.length} MESH &amp; GOSSIP]</span> Node <span class="hi">${target.node}</span> joined private channel <span class="focus">${channelName}</span> <span class="dim">[${channelFp || "live"}]</span> using Argon2id KDF + XChaCha20-Poly1305 encrypted gossip frames.`;
            line2 = `Probe #${cycleNum} MSH ${checkMark(probeCheck("MSH"))}: <span class="hi">${(probeCheck("MSH") || {}).detail || "waiting for the first probe"}</span>; <span class="hi">${activeLinks}/${expectedLinks} encrypted links</span> (beat <span class="hi">#${target.counter || 0}</span> via ${pathMode} <span class="hi">${cleanIp}</span>).`;
      break;
    case 1: // BLB: SHA-256 Content-Addressed Blob Store
          {
      const sto = probeCheck("STO");
      const pulls = Array.isArray(tprobe.pulls) ? tprobe.pulls : [];
      const pullText = pulls.length
        ? pulls.map(p => `<span class="hi">${p.peer}</span> ${okMark(p.ok)} ${Number(p.ms).toFixed(0)}ms`).join(", ")
        : `<span class="dim">no full probe yet (dialtone probe --all)</span>`;
      line1 = `<span class="focus">[2/${TOUR_STEPS.length} BLOB STORE &amp; PULLS]</span> Probe #${cycleNum} on <span class="hi">${target.node}</span> wrote a fresh blob and read it back ${checkMark(sto)} ${sto && sto.ms != null ? `in <span class="hi">${Number(sto.ms).toFixed(0)}ms</span>` : ""} (<span class="hi">${blobKeys} blobs</span>, <span class="hi">${fmtBytesShort(blobBytes)}</span> in the store).`;
      line2 = `A full probe pulls a fresh blob from up to 3 sampled peers over QUIC and verifies its SHA-256 ${checkMark(probeCheck("PUL"))}: ${pullText}.`;
    }
      break;
    case 2: // PRC: Process Load & Exec from Blob Store
      line1 = `<span class="focus">[3/${TOUR_STEPS.length} PROCESS LOAD &amp; EXEC]</span> Loaded verified executable from SHA-256 blob store (<span class="hi">sha256:${shortSha}</span>) &rarr; staged to <span class="hi">${stagedPath}</span>.`;
            line2 = `Supervisor runs <span class="focus">${programName}</span> (<span class="hi">PID ${processPid}</span>) from its blob ${checkMark(probeCheck("PRC"))}; the node checks the mesh every 30s and runs full probes on request (probe <span class="hi">${passCnt}/${totCnt}</span>).`;
      break;
    case 3: // NET: QUIC Transport & RTT Matrix
          {
      // Per-peer results stay on the node that ran the probe (beats leave them out), so a
      // peer's HUD shows the check's summary instead.
      const beats = Array.isArray(tprobe.beats) ? tprobe.beats : [];
      const shown = beats.slice(0, 6);
      const beatText = shown.length
        ? shown.map(b => `<span class="hi">${b.peer}</span> ${okMark(b.ok)} ${b.age_secs}s${b.hears_all ? "" : " (does not hear all)"}`).join(", ")
          + (beats.length > shown.length ? ` and ${beats.length - shown.length} more` : "")
        : `<span class="hi">${(probeCheck("BEAT") || {}).detail || "every peer beating"}</span>`;
      const nbr = probeCheck("NBR");
      line1 = `<span class="focus">[4/${TOUR_STEPS.length} BEATS &amp; RTT]</span> Beats both ways ${checkMark(probeCheck("BEAT"))}: ${beatText}; gossip neighbors ${checkMark(nbr)} <span class="hi">${Object.keys(target.peer_rtt_ms || {}).length}</span>.`;
      line2 = `Rtt from <span class="hi">${target.node}</span> to its gossip neighbors: <span class="focus">${peerRttEntries}</span> (mesh avg <span class="hi">${avgRtt.toFixed(1)}ms ±${avgJitter.toFixed(1)}ms</span>).`;
    }
      break;
    case 4: { // DEV: devices connected to the node, from its inventory
      const devs = Array.isArray(target.devices) ? target.devices : [];
      const counts = {};
      for (const d of devs) counts[d.kind] = (counts[d.kind] || 0) + 1;
      const summary = Object.entries(counts)
        .map(([kind, n]) => `<span class="hi">${n}</span> ${kind}${n > 1 ? "s" : ""}`)
        .join(", ");
      const source = target.os === "macos" ? "system_profiler" : (target.os === "windows" ? "Get-PnpDevice" : "sysfs");
      const featured = devs
        .filter(d => ["camera", "microphone", "speaker", "keyboard", "pointer", "printer", "gamepad"].includes(d.kind))
        .slice(0, 4)
        .map(d => `<span class="hi">${d.name}</span>`)
        .join(" • ");
      line1 = `<span class="focus">[5/${TOUR_STEPS.length} CONNECTED DEVICES]</span> Node <span class="hi">${target.node}</span> lists <span class="hi">${devs.length} devices</span> via ${source}${summary ? ": " + summary : " (none reported yet)"}.`;
      line2 = `${featured ? featured + ". " : ""}Devices travel in the node's signed inventory: listed every 60s and gossiped when they change.`;
      break;
    }
  }

  ccBodyEl.innerHTML = `<div>${line1}</div><div>${line2}</div>`;
}

document.getElementById("tour-cc-header").addEventListener("click", (e) => {
  const stepBtn = e.target.closest("[data-tour-step]");
  if (stepBtn) {
    selectTourStepManual(Number(stepBtn.dataset.tourStep) || 0, performance.now());
    return;
  }
  const navBtn = e.target.closest("[data-tour-nav]");
  if (navBtn) {
    stepTour(Number(navBtn.dataset.tourNav) || 1, performance.now());
    return;
  }
  const toggleBtn = e.target.closest("[data-tour-toggle]");
  if (toggleBtn) {
    tourPlaying = !tourPlaying;
    lastTourSwitchMs = performance.now();
    renderTopShellStats();
    syncMenuControls();
  }
});

document.getElementById("node-top-hud").addEventListener("click", (e) => {
  const row = e.target.closest(".top-row");
  if (row && row.dataset.node) {
    focusNodeManual(row.dataset.node);
  }
});

document.getElementById("node-top-hud").addEventListener("dblclick", (e) => {
  const row = e.target.closest(".top-row");
  const nextLayer = currentZoomLayer < 5 ? currentZoomLayer + 1 : 1;
  if (row && row.dataset.node) {
    focusNodeManual(row.dataset.node, nextLayer);
  } else {
    setUserZoomLayer(nextLayer);
  }
});

// ---------------------------------------------------------------------------
// 9. Thumb Button & Full-Screen Menu Interactions
// ---------------------------------------------------------------------------
const thumbBtn = document.getElementById("thumb-menu-btn");
const fullscreenMenu = document.getElementById("fullscreen-menu");

function toggleFullscreenMenu(forceOpen = null) {
  const isOpen = forceOpen !== null ? forceOpen : fullscreenMenu.hasAttribute("hidden");
  if (isOpen) {
    fullscreenMenu.removeAttribute("hidden");
    thumbBtn.setAttribute("aria-expanded", "true");
    syncMenuControls();
  } else {
    fullscreenMenu.setAttribute("hidden", "");
    thumbBtn.setAttribute("aria-expanded", "false");
  }
}

thumbBtn.addEventListener("click", () => toggleFullscreenMenu());

function syncMenuControls() {
  const nodes = meshState.nodes || [];
  const playBtn = document.getElementById("tour-playback");
  const tourStatus = document.getElementById("menu-tour-status");
  if (playBtn && tourStatus) {
    playBtn.setAttribute("aria-pressed", String(tourPlaying));
    playBtn.classList.toggle("is-focus", tourPlaying);
    playBtn.textContent = tourPlaying ? "▶ TOUR LOOP: ON (PAUSE)" : "■ TOUR PAUSED (PLAY LOOP)";
    tourStatus.textContent = tourPlaying ? "PLAYING ENDLESS LOOP" : "PAUSED (MANUAL)";
  }

  // Layer mode buttons
  document.querySelectorAll("#layer-btn-row button").forEach(btn => {
    const active = Number(btn.dataset.layerMode) === currentZoomLayer;
    btn.setAttribute("aria-pressed", String(active));
    btn.classList.toggle("is-focus", active);
  });
  const layerReadout = document.getElementById("menu-layer-readout");
  if (layerReadout) {
    const lSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[0];
    layerReadout.textContent = `L${lSpec.layer} ${lSpec.name}`;
  }

  // Node selector buttons
  const nodeCountEl = document.getElementById("menu-node-count");
  if (nodeCountEl) nodeCountEl.textContent = `${nodes.length} ACTIVE`;
  const nodeWrap = document.getElementById("menu-node-buttons");
  if (nodeWrap) {
    nodeWrap.innerHTML = "";
    for (const n of nodes) {
      const h = classifyNodeHealth(n);
      const isSel = n.node === selectedNode;
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = `ctrl-btn${isSel ? " is-focus" : ""}`;
      btn.setAttribute("aria-pressed", String(isSel));
      btn.dataset.severity = h.severity;
      const dotCls = h.severity === "error" ? "error" : (h.severity === "warn" ? "warn" : (isSel ? "focus" : ""));
      const hr = n.stats || {};
      btn.innerHTML = `<span class="node-dot ${dotCls}"></span><span>${n.node}</span><span style="color:var(--ink-2)">${(hr.cpu_percent || 0).toFixed(0)}%</span>`;
      btn.addEventListener("click", () => {
        tourPlaying = false;
        focusNodeManual(n.node);
      });
      nodeWrap.appendChild(btn);
    }
  }

  // Mesh state label
  const meshLabel = document.getElementById("menu-mesh-state-label");
  if (meshLabel) {
    const mcfg = meshState.mesh_config || {};
    const cfgOk = Boolean(mcfg.configured ?? meshState.channel_configured);
    const ch = mcfg.channel || meshState.channel_id || "dialtone";
    meshLabel.textContent = cfgOk
      ? `ACTIVE CHANNEL: ${ch.toUpperCase()} (${mcfg.channel_fingerprint || "live"})`
      : "STANDBY (UNCONFIGURED)";
  }
}

// Menu button handlers
document.getElementById("tour-playback").addEventListener("click", () => {
  tourPlaying = !tourPlaying;
  lastTourSwitchMs = performance.now();
  renderTopShellStats();
  syncMenuControls();
});

document.getElementById("btn-prev-stop").addEventListener("click", () => {
  stepTour(-1, performance.now());
});

document.getElementById("btn-next-stop").addEventListener("click", () => {
  stepTour(1, performance.now());
});

document.getElementById("btn-reset-cam").addEventListener("click", () => {
  userDragOffset.identity();
  focusNodeManual(selectedNode, 1);
});

document.querySelectorAll("#layer-btn-row button").forEach(btn => {
  btn.addEventListener("click", () => {
    setUserZoomLayer(Number(btn.dataset.layerMode) || 1);
  });
});

// Theme toggle (deployrobotic dark / light greyscale)
document.getElementById("menu-theme-btn").addEventListener("click", (e) => {
  const root = document.documentElement;
  const next = root.dataset.theme === "dark" ? "light" : "dark";
  root.dataset.theme = next;
  e.currentTarget.textContent = `THEME: ${next.toUpperCase()}`;
  palette = readThemePalette();
  limbMat.uniforms.uColor.value.set(palette.ink2Hex);
  graticuleMat.color.copy(palette.line);
  continentsMat.color.copy(palette.ink);
  landOutlinesMat.color.copy(palette.ink2);
  atmosphereMat.uniforms.uColor.value.set(palette.ink2Hex);
  rebuildGlobeNodesAndArcs(true);
});

// Sliders
document.getElementById("slider-zoom-layer").addEventListener("input", (e) => {
  const val = parseFloat(e.target.value);
  continuousCameraLayer = val;
  const nearest = Math.round(val);
  currentZoomLayer = Math.max(1, Math.min(5, nearest));
  layerMode = currentZoomLayer;
  const lSpec = ZOOM_LAYERS.find(l => l.layer === currentZoomLayer) || ZOOM_LAYERS[2];
  // Interpolate camera distance smoothly between layers
  const lowIdx = Math.max(0, Math.min(4, Math.floor(val) - 1));
  const highIdx = Math.max(0, Math.min(4, Math.ceil(val) - 1));
  const frac = val - Math.floor(val);
  targetCameraDist = THREE.MathUtils.lerp(ZOOM_LAYERS[lowIdx].camDist, ZOOM_LAYERS[highIdx].camDist, frac);
  document.getElementById("val-zoom-layer").textContent = `${val.toFixed(1)} (${lSpec.name})`;
  renderTopShellStats();
  syncMenuControls();
});

document.getElementById("slider-tour-dwell").addEventListener("input", (e) => {
  const sec = parseFloat(e.target.value);
  cfgTourDwellMs = sec * 1000;
  document.getElementById("val-tour-dwell").textContent = `${sec.toFixed(1)}s`;
});

document.getElementById("slider-factory-scale").addEventListener("input", (e) => {
  cfgFactoryScale = parseFloat(e.target.value);
  document.getElementById("val-factory-scale").textContent = `${cfgFactoryScale.toFixed(1)}x`;
  rebuildGlobeNodesAndArcs(true);
});

document.getElementById("slider-land-ink").addEventListener("input", (e) => {
  cfgLandInk = parseFloat(e.target.value) / 100;
  document.getElementById("val-land-ink").textContent = `${Math.round(cfgLandInk * 100)}%`;
});

document.getElementById("slider-cloud-opacity").addEventListener("input", (e) => {
  cfgCloudOpacity = parseFloat(e.target.value) / 100;
  document.getElementById("val-cloud-opacity").textContent = `${Math.round(cfgCloudOpacity * 100)}%`;
});

document.getElementById("slider-packet-speed").addEventListener("input", (e) => {
  cfgPacketSpeed = parseFloat(e.target.value);
  document.getElementById("val-packet-speed").textContent = `${cfgPacketSpeed.toFixed(1)}x`;
});

document.getElementById("slider-term-opacity").addEventListener("input", (e) => {
  const pct = parseFloat(e.target.value);
  document.getElementById("val-term-opacity").textContent = `${Math.round(pct)}%`;
  document.getElementById("terminal-overlay").style.opacity = String(pct / 100);
});

// Toggle buttons
function bindToggle(id, labelPrefix, getter, setter) {
  const btn = document.getElementById(id);
  btn.addEventListener("click", () => {
    const next = !getter();
    setter(next);
    btn.setAttribute("aria-pressed", String(next));
    btn.textContent = `${labelPrefix}: ${next ? "ON" : "OFF"}`;
    rebuildGlobeNodesAndArcs(true);
  });
}
bindToggle("toggle-h3-land", "H3 LAND", () => cfgShowLand, v => { cfgShowLand = v; });
bindToggle("toggle-quic-arcs", "QUIC ARCS", () => cfgShowArcs, v => { cfgShowArcs = v; });
bindToggle("toggle-clouds", "CLOUDS", () => cfgShowClouds, v => { cfgShowClouds = v; });
bindToggle("toggle-drones", "SYMBOL ACTIVITY", () => cfgShowActivity, v => { cfgShowActivity = v; });

// ---------------------------------------------------------------------------
// 10. Pointer Orbit, Raycast Picking & Frame Loop
// ---------------------------------------------------------------------------
window.addEventListener("resize", () => {
  const w = window.innerWidth;
  const h = window.innerHeight;
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  renderer.setSize(w, h, false);
  camera.aspect = w / Math.max(h, 1);
  camera.updateProjectionMatrix();
  limbMat.uniforms.uResolution.value.set(w, h);
});

canvas.addEventListener("pointerdown", (e) => {
  pointerDownPos = { x: e.clientX, y: e.clientY };
  lastMouse = { x: e.clientX, y: e.clientY };
  dragging = true;
});

window.addEventListener("pointermove", (e) => {
  if (!dragging) return;
  const dx = e.clientX - lastMouse.x;
  const dy = e.clientY - lastMouse.y;
  lastMouse = { x: e.clientX, y: e.clientY };
  const qYaw = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(0, 1, 0), dx * 0.0055);
  const qPitch = new THREE.Quaternion().setFromAxisAngle(new THREE.Vector3(1, 0, 0), dy * 0.0055);
  userDragOffset.premultiply(qYaw).premultiply(qPitch);
  lastTourSwitchMs = performance.now();
});

window.addEventListener("pointerup", (e) => {
  if (!dragging) return;
  dragging = false;
  if (Math.hypot(e.clientX - pointerDownPos.x, e.clientY - pointerDownPos.y) < 6) {
    const rect = canvas.getBoundingClientRect();
    pointerNDC.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    pointerNDC.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
    raycaster.setFromCamera(pointerNDC, camera);
    const pickables = siteVisuals.flatMap(sv =>
      sv.labelSprite ? [sv.hitTarget, sv.labelSprite] : [sv.hitTarget]
    );
    const hits = raycaster.intersectObjects(pickables, false);
    if (hits.length > 0 && hits[0].object.userData && hits[0].object.userData.node) {
      focusNodeManual(hits[0].object.userData.node);
    }
  }
});

function cameraDistToContinuousLayer(dist) {
  if (dist >= ZOOM_LAYERS[0].camDist) return 1.0;
  for (let i = 0; i < ZOOM_LAYERS.length - 1; i++) {
    const hi = ZOOM_LAYERS[i];
    const lo = ZOOM_LAYERS[i + 1];
    if (dist <= hi.camDist && dist >= lo.camDist) {
      const t = (hi.camDist - dist) / Math.max(0.001, hi.camDist - lo.camDist);
      return hi.layer + t;
    }
  }
  return 5.0;
}

// Double-click on canvas or node symbol triggers zoom in (1 -> 2 -> 3 -> 4 -> 5, or resets to 1 at max zoom)
canvas.addEventListener("dblclick", (e) => {
  const rect = canvas.getBoundingClientRect();
  pointerNDC.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
  pointerNDC.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;
  raycaster.setFromCamera(pointerNDC, camera);
  const pickables = siteVisuals.map(sv => sv.hitTarget);
  const hits = raycaster.intersectObjects(pickables, false);
  const nextLayer = currentZoomLayer < 5 ? currentZoomLayer + 1 : 1;
  if (hits.length > 0 && hits[0].object.userData && hits[0].object.userData.node) {
    focusNodeManual(hits[0].object.userData.node, nextLayer);
  } else {
    setUserZoomLayer(nextLayer);
  }
});

// Arrow keys trigger zoom in/out (ArrowUp / ArrowDown) or step between nodes (ArrowLeft / ArrowRight)
window.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !fullscreenMenu.hasAttribute("hidden")) {
    toggleFullscreenMenu(false);
    return;
  }
  if (e.target && (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA")) {
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    setUserZoomLayer(Math.min(5, currentZoomLayer + 1));
  } else if (e.key === "ArrowDown") {
    e.preventDefault();
    setUserZoomLayer(Math.max(1, currentZoomLayer - 1));
  } else if (e.key === "ArrowRight") {
    e.preventDefault();
    stepTour(1, performance.now());
  } else if (e.key === "ArrowLeft") {
    e.preventDefault();
    stepTour(-1, performance.now());
  }
});

let prevTimeMs = performance.now();

function renderFrame(nowMs) {
  const delta = Math.min((nowMs - prevTimeMs) / 1000, 0.05);
  prevTimeMs = nowMs;
  const elapsed = nowMs / 1000;

  // Advance endless system tour loop when playing (steady overhead geospatial stop per node, no camera zoom pumping)
  const stepDwellMs = Math.max(2600, Math.round(cfgTourDwellMs * 0.48));
  const cycleMs = (tourStepChangedNode ? cfgTourFlightMs : 0) + stepDwellMs;
  if (tourPlaying && !dragging && (nowMs - lastTourSwitchMs) >= cycleMs) {
    stepTour(1, nowMs);
  }

  // Highlight concentric ring corresponding to the active tour step (1..4)
  const stepMeta = TOUR_STEPS[activeTourStep] || TOUR_STEPS[0];
  activeScanRing = stepMeta.ring;
  const continuousScanRing = stepMeta.ring + 0.18 * Math.sin(elapsed * 3.2);

  // Simple, rock-steady overhead geospatial camera (directly above focused node, North-Up, no bounce or wobble)
  const targetVisual = siteVisuals.find(sv => sv.node === selectedNode) || siteVisuals[0];
  if (targetVisual && !dragging) {
    const flightRaw = Math.min(1, Math.max(0, (nowMs - tourFlightStartMs) / cfgTourFlightMs));
    const flightEased = easeInOutCubic(flightRaw);

    const destQuat = targetQuatForGeo(targetVisual.geo);
    destQuat.premultiply(userDragOffset);

    if (flightRaw < 1.0) {
      globeSurface.quaternion.copy(tourFromQuat).slerp(destQuat, flightEased);
    } else {
      globeSurface.quaternion.copy(destQuat);
    }

    currentCameraDist = THREE.MathUtils.lerp(currentCameraDist, targetCameraDist, 1 - Math.exp(-delta * 4.5));
    camera.position.set(0, 0, currentCameraDist);
    camera.lookAt(0, 0, 0);
  } else if (dragging && targetVisual) {
    const destQuat = targetQuatForGeo(targetVisual.geo);
    destQuat.premultiply(userDragOffset);
    globeSurface.quaternion.slerp(destQuat, 0.35);
    currentCameraDist = THREE.MathUtils.lerp(currentCameraDist, targetCameraDist, 1 - Math.exp(-delta * 4.5));
    camera.position.set(0, 0, currentCameraDist);
    camera.lookAt(0, 0, 0);
  }

  // Continuous zoom layer derived directly from live camera distance
  continuousCameraLayer = cameraDistToContinuousLayer(currentCameraDist);
  const globeOutlineWeight = zoomLayerOpacity(1, continuousCameraLayer, "outline");
  const globeDetailWeight = zoomLayerOpacity(1, continuousCameraLayer, "detail");

  continentsMesh.visible = cfgShowLand;
  landOutlines.visible = cfgShowLand;
  continentsMat.opacity = 0.08 * cfgLandInk * globeDetailWeight;
  landOutlinesMat.opacity = 0.48 * cfgLandInk * globeOutlineWeight;
  graticuleLines.visible = globeDetailWeight > 0.02;
  graticuleMat.opacity = 0.36 * globeDetailWeight;

  cloudsMesh.visible = cfgShowClouds;
  cloudUniforms.uTime.value = elapsed;
  cloudUniforms.uOpacity.value = 0.22 * cfgCloudOpacity * globeDetailWeight;
  atmosphereMesh.visible = cfgShowClouds;
  atmosphereMat.uniforms.uStrength.value = 0.24 * cfgCloudOpacity * globeOutlineWeight;

  // Keep globe centered at (0,0,0) on the camera's optical axis so the focused symbol never drifts off-center when zooming
  globeRoot.position.set(0, 0, 0);

  // Screen-space zoom compensation factor so ping-pong dots and ping circles stay small at every zoom level
  const surfaceDist = Math.max(0.12, currentCameraDist - GLOBE_RADIUS);
  const zoomInvariantDotScale = Math.min(1.15, surfaceDist / 1.7);

  // Animate each Tactical Battlefield Symbol & Concentric Ring Scan Wave
  siteVisuals.forEach((sv, idx) => {
    const rotSpeed = cfgShowActivity ? (0.45 + sv.cpuPct * 0.03) : 0;
    for (const rotor of sv.rotors) {
      rotor.rotation.y = elapsed * rotSpeed;
    }

    if (sv.centerDot) {
      sv.centerDot.scale.set(zoomInvariantDotScale, 1, zoomInvariantDotScale);
    }

    // Progressive nested ring disclosure + in-place Concentric Ring Scan Wave on the focused node
    if (sv.nestedRings) {
      for (const nr of sv.nestedRings) {
        const rawReveal = clamp01((continuousCameraLayer - nr.minLayer) / 0.55);
        // Focused node shows all 4 concentric rings and sweeps a radial highlight wave (Ring 1 -> 2 -> 3 -> 4)
        const distFromWave = Math.abs(nr.ringIdx - continuousScanRing);
        const waveHighlight = clamp01(1.0 - distFromWave * 1.15);
        const focusedRingWeight = 0.42 + 0.58 * waveHighlight;
        const focusFactor = sv.isFocused ? focusedRingWeight : (nr.minLayer <= 2.1 ? 0.32 : 0.0);
        const reveal = easeInOutCubic(rawReveal) * focusFactor;
        nr.group.visible = reveal > 0.015;
        if (nr.group.visible) {
          const baseScale = 0.90 + 0.10 * easeInOutCubic(rawReveal);
          const scanPulse = sv.isFocused ? (1.0 + 0.035 * waveHighlight) : 1.0;
          const s = baseScale * scanPulse;
          nr.group.scale.set(s, 1, s);
          for (const item of nr.tracked) {
            item.mat.opacity = Math.min(1.0, item.baseOpacity * reveal * (sv.isFocused && nr.ringIdx === activeScanRing ? 1.25 : 1.0));
          }
        }
      }
    }

    sv.leds.forEach((ledMat, ledIdx) => {
      if (!cfgShowActivity) return;
      const wave = 0.55 + 0.45 * Math.sin(elapsed * (2.6 + ledIdx * 0.65 + sv.cpuPct * 0.04) + idx * 1.7);
      ledMat.opacity *= wave;
    });

    if (sv.pulseRing) {
      sv.pulseRing.visible = cfgShowActivity;
      const p = (elapsed * 0.55 + idx * 0.33) % 1.0;
      const s = (0.75 + p * 0.65) * zoomInvariantDotScale;
      sv.pulseRing.scale.set(s, 1, s);
      sv.pulseRing.material.opacity = (1.0 - p) * (sv.isFocused ? 0.75 : 0.35);
    }

    if (sv.droneOrbitGroup) {
      sv.droneOrbitGroup.visible = cfgShowActivity;
      sv.droneOrbitGroup.rotation.y = elapsed * (sv.isFocused ? 0.85 : 0.5) + idx;
    }

    if (sv.focusBracket && sv.focusBracket.parent && sv.focusBracket.parent.visible) {
      sv.focusBracket.material.opacity *= (0.7 + 0.3 * Math.sin(elapsed * 3.8));
    }
  });

  // Replay real blob / process FX for the focused node whenever the tour lands on BLB or PRC
  const tourFxKey = `${activeTourStep}|${selectedNode}|${lastTourSwitchMs}`;
  if (tourFxKey !== lastTourFxKey) {
    lastTourFxKey = tourFxKey;
    replayTourStepFx(activeTourStep, selectedNode);
  }
  // Live event FX keep a roughly constant on-screen size across zoom layers
  updateLiveFx(nowMs, Math.max(0.3, Math.min(1.15, surfaceDist / 5.0)));

  // Animate QUIC packets along great-circle arcs (scaled by zoomInvariantDotScale so they are always small in every zoom)
  for (const av of arcVisuals) {
    const baseSpeed = Math.max(0.22, Math.min(1.1, 14.0 / Math.max(4.0, av.rtt))) * cfgPacketSpeed;
    const tFwd = (elapsed * baseSpeed + av.phase) % 1.0;
    const tRev = (1.0 - tFwd + 1.0) % 1.0;
    const idxFwd = Math.min(av.pts.length - 1, Math.floor(tFwd * av.pts.length));
    const idxRev = Math.min(av.pts.length - 1, Math.floor(tRev * av.pts.length));
    av.pktFwd.position.copy(av.pts[idxFwd]);
    av.pktRev.position.copy(av.pts[idxRev]);
    av.pktFwd.scale.setScalar(zoomInvariantDotScale);
    av.pktRev.scale.setScalar(zoomInvariantDotScale);
  }

  renderer.render(scene, camera);
  requestAnimationFrame(renderFrame);
}

// Beats carry only what every node needs. The focused node's details (devices, full probe,
// processes with their status, services, rolling summary, store, full stats and transport)
// are fetched when it is focused (`/api/details`, which asks the node itself).
const nodeDetails = new Map(); // node_id -> { at, pending, ...details }
const DETAIL_FIELDS = ["devices", "processes", "service_endpoints", "summary_stats", "store", "stats", "iroh_transport", "os", "arch"];
const DETAILS_REFRESH_MS = 10000;

async function refreshFocusDetails() {
  const node = (meshState.nodes || []).find(n => n.node === selectedNode);
  if (!node || !node.node_id) return;
  const known = nodeDetails.get(node.node_id) || { at: -Infinity };
  if (known.pending || performance.now() - known.at < DETAILS_REFRESH_MS) return;
  nodeDetails.set(node.node_id, { ...known, pending: true });
  let fetched = {};
  try {
    const resp = await fetch(`/api/details?node_id=${encodeURIComponent(node.node_id)}`, { cache: "no-store" });
    if (resp.ok) fetched = scrubStrings(await resp.json());
  } catch (_) {}
  nodeDetails.set(node.node_id, { ...known, ...fetched, pending: false, at: performance.now() });
  renderTopShellStats();
}

// A node with its fetched details laid over what gossip brought; the newer probe wins.
function withDetails(node) {
  const d = node && nodeDetails.get(node.node_id);
  if (!d) return node;
  const probe = d.probe && (!node.probe || (d.probe.cycle || 0) >= (node.probe.cycle || 0)) ? d.probe : node.probe;
  const out = { ...node, probe };
  for (const k of DETAIL_FIELDS) {
    if (d[k] !== undefined && d[k] !== null) out[k] = d[k];
  }
  return out;
}

async function pollGossipState() {
  try {
    const resp = await fetch("/api/mesh", { cache: "no-store" });
    if (!resp.ok) return;
    const data = await resp.json();
    meshState = scrubStrings(data);
    if (!meshState.nodes.some(n => n.node === selectedNode) && meshState.nodes.length > 0) {
      selectedNode = meshState.self_node || meshState.nodes[0].node;
      stepTour(0, performance.now());
    } else {
      renderTopShellStats();
      if (!fullscreenMenu.hasAttribute("hidden")) {
        syncMenuControls();
      }
    }
    detectNodeFxEvents(meshState.nodes || []);
    refreshFocusDetails();
  } catch (_) {}
}

tourStops = buildTourSequence();
if (tourStops.length > 0) {
  const startIdx = Math.max(0, tourStops.findIndex(s => s.node === selectedNode));
  tourStopIndex = startIdx;
  applyStop(tourStops[tourStopIndex], performance.now());
} else {
  renderTopShellStats();
}
detectNodeFxEvents(meshState.nodes || [], false);
requestAnimationFrame(renderFrame);
setInterval(pollGossipState, 1500);
