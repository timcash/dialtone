import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { DagStageCamera, DagCameraView } from './camera';

type NodeID = string;
type EdgeID = string;
type LayerID = string;

type DagNode = {
  id: NodeID;
  label: string;
  layerId: LayerID;
  rank: number;
  row: number;
  mesh: THREE.Mesh;
  labelTexture: THREE.CanvasTexture | null;
  labelCanvas: HTMLCanvasElement | null;
  labelMesh: THREE.Mesh;
  nestedLayerIDs: LayerID[];
};

type DagEdge = {
  id: EdgeID;
  layerId: LayerID;
  outputNodeId: NodeID;
  inputNodeId: NodeID;
  line: THREE.Line;
};

type DagLayer = {
  id: LayerID;
  parentNodeId?: NodeID;
  baseX: number;
  baseY: number;
  baseZ: number;
  anchor: THREE.Group;
  grid: THREE.GridHelper;
  nodeIds: NodeID[];
  edgeIds: EdgeID[];
};

type LayerSnapshot = {
  layerId: LayerID;
  selectedNodeId: NodeID;
};

type ProjectedPoint = { ok: boolean; x: number; y: number };
type CameraView = DagCameraView;
type ThumbMode = 'graph' | 'layer' | 'camera';
type ThumbActionID =
  | 'back'
  | 'add'
  | 'link_or_unlink'
  | 'open_or_close_layer'
  | 'clear_picks'
  | 'rename'
  | 'camera_top'
  | 'camera_iso'
  | 'camera_side'
  | 'toggle_labels'
  | 'focus'
  | 'none';
type ThumbActionDef = { id: ThumbActionID; label: string };
type NestedLink = {
  parentNodeId: NodeID;
  childNodeId: NodeID;
  childLayerId: LayerID;
  line: THREE.Line;
};

const NODE_BASE_COLOR = 0x475261;
const NODE_RECENT_COLOR = 0xf3f8ff;
const NODE_SECOND_RECENT_COLOR = 0x2b78ff;
const EDGE_BASE_COLOR = 0x6b7280;
const EDGE_NESTED_LINK_COLOR = 0xfacc15;
const ROOT_IO_CAMERA_VIEW: CameraView = 'iso';
const CHATLOG_MAX_LINES = 7;

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 400);
  private stageCamera = new DagStageCamera(this.camera);
  private renderer: THREE.WebGLRenderer;
  private gizmoScene = new THREE.Scene();
  private gizmoCamera = new THREE.PerspectiveCamera(50, 1, 0.1, 10);
  private gizmoAxes = new THREE.AxesHelper(0.8);
  private raycaster = new THREE.Raycaster();
  private pointer = new THREE.Vector2(2, 2);
  private frameID = 0;
  private visible = false;

  private rankXSpacing = 8;
  private rowZSpacing = 5.5;
  private nestedLayerYOffset = 15;

  private layers = new Map<LayerID, DagLayer>();
  private nodes = new Map<NodeID, DagNode>();
  private edges = new Map<EdgeID, DagEdge>();
  private selectedNodeId = '';
  private activeLayerId: LayerID = 'root';
  private history: LayerSnapshot[] = [];
  private openedLayerIDs = new Set<LayerID>();
  private allMeshes: THREE.Mesh[] = [];
  private keyLight: THREE.DirectionalLight;
  private nestedLinks: NestedLink[] = [];
  private cameraView: CameraView = ROOT_IO_CAMERA_VIEW;

  private userNodeCounter = 1;
  private edgeCounter = 1;
  private recentSelectedNodeIDs: NodeID[] = [];
  private labelsVisible = true;
  private lastCreatedNodeId = '';
  private renameInput: HTMLInputElement | null = null;
  private renameApplyButton: HTMLButtonElement | null = null;
  private modeButton: HTMLButtonElement | null = null;
  private thumbButtons: HTMLButtonElement[] = [];
  private thumbMode: ThumbMode = 'graph';
  private chatlogHost: HTMLElement | null = null;
  private chatlogTerm: Terminal | null = null;
  private chatlogLines: string[] = [];
  private readonly modeOrder: ThumbMode[] = ['graph', 'layer', 'camera'];
  private readonly modeLabels: Record<ThumbMode, string> = {
    graph: 'Mode: Graph',
    layer: 'Mode: Layer',
    camera: 'Mode: Camera',
  };
  private nodeHistoryLabelEl: HTMLElement | null = null;
  private nodeHistoryValueEls: HTMLElement[] = [];

  constructor(private container: HTMLElement, private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x05070a, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.autoClear = false;

    this.camera.position.set(0, 22, 28);
    this.camera.lookAt(0, 0, 0);

    this.scene.add(new THREE.AmbientLight(0xffffff, 0.45));
    this.keyLight = new THREE.DirectionalLight(0xffffff, 0.95);
    this.keyLight.position.set(16, 22, 18);
    this.scene.add(this.keyLight);
    this.initGizmo();

    this.bootstrapGraph();
    this.applyLayerView('root');
    this.initMobileUI();
    this.attachEvents();
    this.attachDebugBridge();
    this.resize();
    this.animate();
    this.canvas.setAttribute('data-ready', 'true');
  }

  private initGizmo() {
    this.gizmoCamera.position.set(0, 0, 2.6);
    this.gizmoCamera.lookAt(0, 0, 0);
    this.gizmoScene.add(new THREE.AmbientLight(0xffffff, 0.8));
    this.gizmoScene.add(this.gizmoAxes);
  }

  private bootstrapGraph() {
    this.createLayer('root', undefined, 0, 0, 0);
  }

  private createLayer(id: LayerID, parentNodeId?: NodeID, baseX = 0, baseY = 0, baseZ = 0) {
    const anchor = new THREE.Group();
    anchor.position.set(baseX, baseY, baseZ);
    anchor.name = `layer_${id}`;
    const grid = this.attachLayerGrid(anchor);
    this.scene.add(anchor);
    this.layers.set(id, { id, parentNodeId, baseX, baseY, baseZ, anchor, grid, nodeIds: [], edgeIds: [] });
  }

  private attachLayerGrid(anchor: THREE.Group): THREE.GridHelper {
    const width = 40;
    const depth = 28;
    const size = Math.max(width, depth);
    const divisions = 32;
    const grid = new THREE.GridHelper(size, divisions, 0x444444, 0x222222);
    grid.position.set(0, -1.1, 0);
    grid.scale.set(width / size, 1, depth / size);
    const material = grid.material as THREE.Material;
    material.transparent = true;
    material.opacity = 0.6;
    anchor.add(grid);
    return grid;
  }

  private setLayerGridOpacity(layer: DagLayer, opacity: number) {
    const materials = Array.isArray(layer.grid.material) ? layer.grid.material : [layer.grid.material];
    for (const material of materials) {
      material.transparent = true;
      material.opacity = opacity;
    }
  }

  private rankToX(rank: number): number {
    return rank * this.rankXSpacing - this.rankXSpacing;
  }

  private rowToZ(row: number): number {
    return -row * this.rowZSpacing;
  }

  private createLabelMesh(text: string): { mesh: THREE.Mesh; texture: THREE.CanvasTexture | null; canvas: HTMLCanvasElement | null } {
    const w = 256;
    const h = 80;
    const canvas = document.createElement('canvas');
    canvas.width = w;
    canvas.height = h;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      const fallback = new THREE.MeshBasicMaterial({ color: 0xffffff, side: THREE.DoubleSide, transparent: true, opacity: 0 });
      return { mesh: new THREE.Mesh(new THREE.PlaneGeometry(3.2, 1), fallback), texture: null, canvas: null };
    }
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = '#d8e9ff';
    ctx.font = '600 34px ui-monospace, SFMono-Regular, Menlo, monospace';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 8, h / 2);
    const texture = new THREE.CanvasTexture(canvas);
    texture.needsUpdate = true;
    const material = new THREE.MeshBasicMaterial({ map: texture, transparent: true, depthWrite: false, side: THREE.DoubleSide });
    return { mesh: new THREE.Mesh(new THREE.PlaneGeometry(3.2, 1), material), texture, canvas };
  }

  private createNode(id: NodeID, layerId: LayerID, rank: number, row: number, nestedLayerIDs: LayerID[] = []) {
    const layer = this.layers.get(layerId);
    if (!layer) throw new Error(`missing layer ${layerId}`);

    const geometry = new THREE.BoxGeometry(1.5, 1.5, 1.5);
    const material = new THREE.MeshStandardMaterial({
      color: NODE_BASE_COLOR,
      emissive: 0x000000,
      emissiveIntensity: 1.0,
      roughness: 0.42,
      metalness: 0.24,
    });
    const mesh = new THREE.Mesh(geometry, material);
    mesh.position.set(this.rankToX(rank), 0, this.rowToZ(row));
    mesh.userData = { nodeId: id };
    layer.anchor.add(mesh);
    const label = id;
    const labelAsset = this.createLabelMesh(label);
    const labelMesh = labelAsset.mesh;
    labelMesh.position.set(this.rankToX(rank) + 2.25, 0, this.rowToZ(row));
    labelMesh.visible = this.labelsVisible;
    layer.anchor.add(labelMesh);

    const node: DagNode = {
      id,
      label,
      layerId,
      rank,
      row,
      mesh,
      labelTexture: labelAsset.texture,
      labelCanvas: labelAsset.canvas,
      labelMesh,
      nestedLayerIDs,
    };
    this.nodes.set(id, node);
    layer.nodeIds.push(id);
    this.allMeshes.push(mesh);
    if (layer.parentNodeId) {
      this.createNestedLink(layer.parentNodeId, id, layer.id);
    }
  }

  private createEdge(id: EdgeID, layerId: LayerID, outputNodeId: NodeID, inputNodeId: NodeID) {
    const outputNode = this.nodes.get(outputNodeId);
    const inputNode = this.nodes.get(inputNodeId);
    if (!outputNode || !inputNode) throw new Error(`edge node missing for ${id}`);

    const layer = this.layers.get(layerId);
    if (!layer) throw new Error(`missing layer ${layerId}`);
    const line = this.newLine(outputNode.mesh.position, inputNode.mesh.position, EDGE_BASE_COLOR);
    layer.anchor.add(line);
    this.edges.set(id, { id, layerId, outputNodeId, inputNodeId, line });
    layer.edgeIds.push(id);
  }

  private createNestedLink(parentNodeId: NodeID, childNodeId: NodeID, childLayerId: LayerID) {
    const parentNode = this.nodes.get(parentNodeId);
    const childNode = this.nodes.get(childNodeId);
    if (!parentNode || !childNode) return;
    const line = this.newLine(
      parentNode.mesh.getWorldPosition(new THREE.Vector3()),
      childNode.mesh.getWorldPosition(new THREE.Vector3()),
      EDGE_NESTED_LINK_COLOR
    );
    this.scene.add(line);
    this.nestedLinks.push({ parentNodeId, childNodeId, childLayerId, line });
  }

  private updateNestedLinkGeometry(link: NestedLink) {
    const parentNode = this.nodes.get(link.parentNodeId);
    const childNode = this.nodes.get(link.childNodeId);
    if (!parentNode || !childNode) return;
    link.line.geometry.dispose();
    link.line.geometry = new THREE.BufferGeometry().setFromPoints([
      parentNode.mesh.getWorldPosition(new THREE.Vector3()),
      childNode.mesh.getWorldPosition(new THREE.Vector3()),
    ]);
  }

  private newLine(from: THREE.Vector3, to: THREE.Vector3, color: number): THREE.Line {
    const points = [from.clone(), to.clone()];
    const geometry = new THREE.BufferGeometry().setFromPoints(points);
    const material = new THREE.LineBasicMaterial({ color });
    return new THREE.Line(geometry, material);
  }

  private updateEdgeLine(edge: DagEdge) {
    const outputNode = this.nodes.get(edge.outputNodeId);
    const inputNode = this.nodes.get(edge.inputNodeId);
    if (!outputNode || !inputNode) return;
    edge.line.geometry.dispose();
    edge.line.geometry = new THREE.BufferGeometry().setFromPoints([outputNode.mesh.position.clone(), inputNode.mesh.position.clone()]);
  }

  private ensureNestedLayer(nodeId: NodeID): LayerID {
    const parentNode = this.nodes.get(nodeId);
    if (!parentNode) return '';
    const existingLayerIDs = parentNode.nestedLayerIDs.filter((layerID) => this.layers.has(layerID));
    if (existingLayerIDs.length > 0) return existingLayerIDs[existingLayerIDs.length - 1];

    const parentLayer = this.layers.get(parentNode.layerId);
    const parentWorld = parentNode.mesh.getWorldPosition(new THREE.Vector3());
    const parentBaseY = parentLayer ? parentLayer.baseY : 0;
    const nestedLayerId = `layer_nested_${nodeId}_${parentNode.nestedLayerIDs.length + 1}`;
    this.createLayer(nestedLayerId, nodeId, parentWorld.x, parentBaseY + this.nestedLayerYOffset, parentWorld.z);
    parentNode.nestedLayerIDs.push(nestedLayerId);
    this.refreshVisualState();
    return nestedLayerId;
  }

  private attachEvents() {
    window.addEventListener('resize', this.resize);
    this.canvas.addEventListener('click', this.onClick);
    this.canvas.addEventListener('dblclick', this.onDoubleClick);
  }

  private onClick = (event: MouseEvent) => {
    const hit = this.hitTestClientPoint(event.clientX, event.clientY);
    if (hit) {
      this.selectNode(hit);
      return;
    }
    this.selectNode('');
  };

  private onDoubleClick = () => {
    if (!this.selectedNodeId) return;
    this.openNestedLayer(this.selectedNodeId);
  };

  private initMobileUI() {
    this.thumbButtons = [
      this.container.querySelector("button[aria-label='DAG Thumb 1']"),
      this.container.querySelector("button[aria-label='DAG Thumb 2']"),
      this.container.querySelector("button[aria-label='DAG Thumb 3']"),
      this.container.querySelector("button[aria-label='DAG Thumb 4']"),
      this.container.querySelector("button[aria-label='DAG Thumb 5']"),
      this.container.querySelector("button[aria-label='DAG Thumb 6']"),
      this.container.querySelector("button[aria-label='DAG Thumb 7']"),
      this.container.querySelector("button[aria-label='DAG Thumb 8']"),
    ].filter((el): el is HTMLButtonElement => !!el);
    this.modeButton = this.container.querySelector("button[aria-label='DAG Mode']");
    this.nodeHistoryLabelEl = this.container.querySelector('.dag-history > h3');
    this.nodeHistoryValueEls = [
      this.container.querySelector("[aria-label='DAG Node History Item 1']"),
      this.container.querySelector("[aria-label='DAG Node History Item 2']"),
      this.container.querySelector("[aria-label='DAG Node History Item 3']"),
      this.container.querySelector("[aria-label='DAG Node History Item 4']"),
      this.container.querySelector("[aria-label='DAG Node History Item 5']"),
    ].filter((el): el is HTMLElement => !!el);
    this.renameInput = this.container.querySelector("input[aria-label='DAG Label Input']");
    this.renameApplyButton = this.container.querySelector("button[aria-label='DAG Rename']");
    this.chatlogHost = this.container.querySelector('.dag-chatlog-xterm');
    this.initChatlogTerminal();
    const testMode = (() => {
      try {
        return window.sessionStorage.getItem('dag_test_mode') === '1';
      } catch {
        return false;
      }
    })();

    for (let i = 0; i < this.thumbButtons.length; i += 1) {
      const button = this.thumbButtons[i];
      button.addEventListener('click', () => {
        this.runThumbActionBySlot(i);
      });
    }
    this.modeButton?.addEventListener('click', () => {
      this.cycleThumbMode();
      this.syncControlState();
    });
    this.renameApplyButton?.addEventListener('click', () => this.applyRenameFromInput());
    this.renameInput?.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') return;
      event.preventDefault();
      this.applyRenameFromInput();
    });
    if (testMode) this.container.setAttribute('data-test-mode', 'true');

    this.syncControlState();
  }

  private initChatlogTerminal() {
    if (!this.chatlogHost) return;
    this.chatlogTerm?.dispose();
    this.chatlogHost.innerHTML = '';
    this.chatlogTerm = new Terminal({
      allowTransparency: true,
      convertEol: true,
      disableStdin: true,
      cursorBlink: false,
      cursorStyle: 'bar',
      rows: CHATLOG_MAX_LINES,
      cols: 92,
      scrollback: 0,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 12,
      lineHeight: 1.35,
      theme: {
        background: 'rgba(0,0,0,0)',
        foreground: '#a7adb7',
        cursor: '#a7adb7',
      },
    });
    this.chatlogTerm.open(this.chatlogHost);
    this.renderChatlog();
  }

  private normalizeThoughtText(text: string): string {
    const single = text.replace(/\s+/g, ' ').trim();
    if (single.length <= 104) return single;
    return `${single.slice(0, 101)}...`;
  }

  private renderChatlog() {
    const term = this.chatlogTerm;
    if (!term) return;
    const lines = this.chatlogLines.slice(-CHATLOG_MAX_LINES);
    const padCount = Math.max(0, CHATLOG_MAX_LINES - lines.length);
    const rendered: string[] = [];
    for (let i = 0; i < padCount; i += 1) rendered.push('');
    for (let i = 0; i < lines.length; i += 1) {
      const age = lines.length - 1 - i;
      const color =
        age === 0 ? '\x1b[97m' : age === 1 ? '\x1b[37m' : age === 2 ? '\x1b[2;37m' : age === 3 ? '\x1b[90m' : '\x1b[2;90m';
      rendered.push(`${color}${lines[i]}\x1b[0m`);
    }
    term.write(`\x1b[2J\x1b[H${rendered.join('\r\n')}`);
  }

  private appendThought(text: string): boolean {
    const clean = this.normalizeThoughtText(text);
    if (!clean) return false;
    this.chatlogLines.push(clean);
    if (this.chatlogLines.length > CHATLOG_MAX_LINES) {
      this.chatlogLines = this.chatlogLines.slice(-CHATLOG_MAX_LINES);
    }
    this.renderChatlog();
    return true;
  }

  private cycleThumbMode() {
    const idx = this.modeOrder.indexOf(this.thumbMode);
    const next = this.modeOrder[(idx + 1) % this.modeOrder.length];
    this.thumbMode = next;
  }

  private getThumbActionsForMode(): ThumbActionDef[] {
    if (this.thumbMode === 'layer') {
      return [
        { id: 'open_or_close_layer', label: this.getOpenCloseLayerActionLabel() },
        { id: 'add', label: 'Add' },
        { id: 'back', label: 'Back' },
        { id: 'clear_picks', label: 'Clear' },
        { id: 'link_or_unlink', label: this.getLinkUnlinkLabel() },
        { id: 'focus', label: 'Focus' },
        { id: 'rename', label: 'Rename' },
        { id: 'none', label: '-' },
      ];
    }
    if (this.thumbMode === 'camera') {
      return [
        { id: 'camera_top', label: 'Z' },
        { id: 'camera_iso', label: 'ISO' },
        { id: 'camera_side', label: 'SIDE' },
        { id: 'focus', label: 'Focus' },
        { id: 'toggle_labels', label: this.labelsVisible ? 'Labels On' : 'Labels Off' },
        { id: 'open_or_close_layer', label: this.getOpenCloseLayerActionLabel() },
        { id: 'back', label: 'Back' },
        { id: 'none', label: '-' },
      ];
    }
    return [
      { id: 'back', label: 'Back' },
      { id: 'add', label: 'Add' },
      { id: 'link_or_unlink', label: this.getLinkUnlinkLabel() },
      { id: 'clear_picks', label: 'Clear' },
      { id: 'open_or_close_layer', label: this.getOpenCloseLayerActionLabel() },
      { id: 'rename', label: 'Rename' },
      { id: 'focus', label: 'Focus' },
      { id: 'toggle_labels', label: this.labelsVisible ? 'Labels On' : 'Labels Off' },
    ];
  }

  private runThumbActionBySlot(slot: number) {
    const action = this.getThumbActionsForMode()[slot];
    if (!action) return;
    const actionID = action.id;
    if (actionID === 'none') return;
    if (actionID === 'back') {
      this.performBackAction();
      return;
    }
    if (actionID === 'add') {
      this.createChildNodeFromSelected();
      this.syncControlState();
      return;
    }
    if (actionID === 'link_or_unlink') {
      this.performLinkOrUnlinkAction();
      this.syncControlState();
      return;
    }
    if (actionID === 'clear_picks') {
      this.clearSelections();
      return;
    }
    if (actionID === 'open_or_close_layer') {
      this.performOpenOrCloseLayerAction();
      this.syncControlState();
      return;
    }
    if (actionID === 'rename') {
      this.applyRenameFromInput();
      return;
    }
    if (actionID === 'camera_top') {
      this.setCameraView('top');
      return;
    }
    if (actionID === 'camera_iso') {
      this.setCameraView('iso');
      return;
    }
    if (actionID === 'camera_side') {
      this.setCameraView('side');
      return;
    }
    if (actionID === 'focus') {
      if (this.selectedNodeId) {
        this.focusCameraOnNode(this.selectedNodeId);
      } else {
        this.fitCameraToLayer(this.activeLayerId);
      }
      this.syncControlState();
      return;
    }
    if (actionID === 'toggle_labels') {
      this.labelsVisible = !this.labelsVisible;
      this.refreshVisualState();
      this.syncControlState();
    }
  }

  private createChildNodeFromSelected() {
    if (!this.selectedNodeId) {
      const nodeId = `n_user_${this.userNodeCounter++}`;
      if (!this.addNode(this.activeLayerId, nodeId, 0, 0)) return;
      this.lastCreatedNodeId = nodeId;
      this.selectNode(nodeId);
      console.log(`[Three #three] action add node: ${nodeId} on layer ${this.activeLayerId}`);
      return;
    }
    const source = this.nodes.get(this.selectedNodeId);
    if (!source) return;
    const nodeId = `n_user_${this.userNodeCounter++}`;
    if (!this.addNode(source.layerId, nodeId, source.rank + 1, source.row)) return;
    this.lastCreatedNodeId = nodeId;
    this.selectNode(nodeId);
    console.log(`[Three #three] action add node: ${nodeId} near ${source.id}`);
  }

  private performBackAction() {
    if (this.history.length > 0) {
      this.navigateBackToParentLayer();
      return;
    }
    this.popSelectionHistoryBack();
  }

  private getLinkUnlinkLabel(): string {
    const pair = this.getRecentLinkPairForActiveLayer();
    if (!pair) return 'link';
    return this.findEdgeIDBetween(pair.outputNodeId, pair.inputNodeId) ? 'unlink' : 'link';
  }

  private getOpenCloseLayerActionLabel(): string {
    const selectedNode = this.nodes.get(this.selectedNodeId);
    if (!selectedNode || selectedNode.layerId !== this.activeLayerId) return 'open';
    const nestedLayerID = this.getExistingNestedLayerID(selectedNode.id);
    if (!nestedLayerID) return 'open';
    if (this.openedLayerIDs.has(nestedLayerID)) return 'close';
    return 'open';
  }

  private performOpenOrCloseLayerAction() {
    const selectedNode = this.nodes.get(this.selectedNodeId);
    if (!selectedNode || selectedNode.layerId !== this.activeLayerId) return;
    const existingLayerID = this.getExistingNestedLayerID(selectedNode.id);
    const nestedLayerID = existingLayerID || this.ensureNestedLayer(selectedNode.id);
    if (!nestedLayerID) return;
    if (this.openedLayerIDs.has(nestedLayerID)) {
      this.closeNestedLayerByID(nestedLayerID);
      return;
    }
    this.openNestedLayer(selectedNode.id);
  }

  private getExistingNestedLayerID(nodeId: NodeID): LayerID {
    const node = this.nodes.get(nodeId);
    if (!node) return '';
    const existing = node.nestedLayerIDs.filter((layerID) => this.layers.has(layerID));
    return existing.length > 0 ? existing[existing.length - 1] : '';
  }

  private getRecentLinkPair(): { outputNodeId: NodeID; inputNodeId: NodeID } | null {
    if (this.recentSelectedNodeIDs.length < 2) return null;
    const inputNodeId = this.recentSelectedNodeIDs[0];
    const outputNodeId = this.recentSelectedNodeIDs[1];
    if (!outputNodeId || !inputNodeId || outputNodeId === inputNodeId) return null;
    return { outputNodeId, inputNodeId };
  }

  private getRecentLinkPairForActiveLayer(): { outputNodeId: NodeID; inputNodeId: NodeID } | null {
    const pair = this.getRecentLinkPair();
    if (!pair) return null;
    const source = this.nodes.get(pair.outputNodeId);
    const target = this.nodes.get(pair.inputNodeId);
    if (!source || !target) return null;
    if (source.layerId !== this.activeLayerId || target.layerId !== this.activeLayerId) return null;
    return pair;
  }

  private clearSelections() {
    this.selectedNodeId = '';
    this.recentSelectedNodeIDs = [];
    this.refreshVisualState();
    this.syncCanvasState();
  }

  private performConnectAction() {
    const pair = this.getRecentLinkPairForActiveLayer();
    if (!pair) return;
    const source = this.nodes.get(pair.outputNodeId);
    const target = this.nodes.get(pair.inputNodeId);
    if (!source || !target) return;
    if (source.id === target.id) return;
    if (!source || !target || source.layerId !== target.layerId) return;
    if (source.layerId !== this.activeLayerId) return;
    if (this.findEdgeIDBetween(source.id, target.id)) {
      return;
    }
    const edgeId = `e_user_${this.edgeCounter++}`;
    if (this.addEdge(edgeId, source.layerId, source.id, target.id)) {
      console.log(`[Three #three] action add edge: ${source.id} -> ${target.id}`);
    }
  }

  private findEdgeIDBetween(outputNodeId: NodeID, inputNodeId: NodeID): EdgeID {
    const layer = this.layers.get(this.activeLayerId);
    if (!layer) return '';
    for (const edgeId of layer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge) continue;
      if (edge.outputNodeId === outputNodeId && edge.inputNodeId === inputNodeId) return edge.id;
    }
    return '';
  }

  private performUnlinkAction() {
    const pair = this.getRecentLinkPairForActiveLayer();
    if (!pair) return;
    const outputNodeId = pair.outputNodeId;
    const inputNodeId = pair.inputNodeId;
    if (!outputNodeId || !inputNodeId) return;
    const edgeID = this.findEdgeIDBetween(outputNodeId, inputNodeId);
    if (!edgeID) return;
    if (this.removeEdge(edgeID)) {
      console.log(`[Three #three] action remove edge: ${outputNodeId} -> ${inputNodeId}`);
    }
  }

  private performLinkOrUnlinkAction() {
    const pair = this.getRecentLinkPairForActiveLayer();
    if (!pair) return;
    const edgeID = this.findEdgeIDBetween(pair.outputNodeId, pair.inputNodeId);
    if (edgeID) {
      this.performUnlinkAction();
      return;
    }
    this.performConnectAction();
  }

  private redrawNodeLabel(node: DagNode) {
    if (!node.labelCanvas || !node.labelTexture) return;
    const ctx = node.labelCanvas.getContext('2d');
    if (!ctx) return;
    const w = node.labelCanvas.width;
    const h = node.labelCanvas.height;
    ctx.clearRect(0, 0, w, h);
    ctx.fillStyle = '#d8e9ff';
    ctx.font = '600 34px ui-monospace, SFMono-Regular, Menlo, monospace';
    ctx.textAlign = 'left';
    ctx.textBaseline = 'middle';
    ctx.fillText(node.label, 8, h / 2);
    node.labelTexture.needsUpdate = true;
  }

  private renameNode(nodeId: NodeID, label: string): boolean {
    const node = this.nodes.get(nodeId);
    if (!node) return false;
    const text = label.trim();
    if (!text) return false;
    node.label = text;
    this.redrawNodeLabel(node);
    this.syncControlState();
    return true;
  }

  private applyRenameFromInput() {
    if (!this.selectedNodeId || !this.renameInput) return;
    if (this.renameNode(this.selectedNodeId, this.renameInput.value)) {
      console.log(`[Three #three] rename node: ${this.selectedNodeId} -> ${this.renameInput.value.trim()}`);
    }
  }

  private syncControlState() {
    const pair = this.getRecentLinkPairForActiveLayer();
    const selectedNode = this.nodes.get(this.selectedNodeId);
    const hasPair = !!pair;
    const canOpenLayer = !!selectedNode && selectedNode.layerId === this.activeLayerId;
    const canRename = !!selectedNode;
    const actions = this.getThumbActionsForMode();

    for (let i = 0; i < this.thumbButtons.length; i += 1) {
      const button = this.thumbButtons[i];
      const action = actions[i] || { id: 'none', label: '-' };
      let disabled = false;
      if (action.id === 'none') disabled = true;
      if (action.id === 'back') disabled = this.history.length === 0 && this.recentSelectedNodeIDs.length < 2;
      if (action.id === 'link_or_unlink') disabled = !hasPair;
      if (action.id === 'clear_picks') disabled = !(this.recentSelectedNodeIDs.length > 0 || this.selectedNodeId);
      if (action.id === 'rename') disabled = !canRename;
      if (action.id === 'open_or_close_layer') disabled = !canOpenLayer;
      button.textContent = action.label;
      button.disabled = disabled;
      button.setAttribute('data-action', action.id);
      const cameraActive =
        (action.id === 'camera_top' && this.cameraView === 'top') ||
        (action.id === 'camera_iso' && this.cameraView === 'iso') ||
        (action.id === 'camera_side' && this.cameraView === 'side');
      button.classList.toggle('is-active', cameraActive);
    }

    if (this.renameInput) {
      this.renameInput.disabled = !selectedNode;
      this.renameInput.value = selectedNode ? selectedNode.label : '';
      this.renameInput.placeholder = selectedNode ? 'Rename selected node' : 'Select node to rename';
    }
    if (this.renameApplyButton) this.renameApplyButton.disabled = !selectedNode;
    if (this.modeButton) {
      this.modeButton.textContent = this.modeLabels[this.thumbMode];
      this.modeButton.setAttribute('data-mode', this.thumbMode);
    }
    for (let i = 0; i < this.nodeHistoryValueEls.length; i += 1) {
      this.nodeHistoryValueEls[i].textContent = this.recentSelectedNodeIDs[i] || 'none';
    }
    if (this.nodeHistoryLabelEl) {
      this.nodeHistoryLabelEl.textContent = `Node History ${this.getCurrentLayerNumber()}/${this.getVisibleLayerCount()}`;
    }
    this.canvas.setAttribute('data-labels-visible', String(this.labelsVisible));
    this.canvas.setAttribute('data-link-output', pair?.outputNodeId ?? '');
    this.canvas.setAttribute('data-link-input', pair?.inputNodeId ?? '');
    this.canvas.setAttribute('data-thumb-mode', this.thumbMode);
  }

  private getCurrentLayerNumber(): number {
    return this.history.length + 1;
  }

  private getVisibleLayerCount(): number {
    const visibleLayerIDs = new Set<LayerID>();
    visibleLayerIDs.add('root');
    visibleLayerIDs.add(this.activeLayerId);
    for (const layerID of this.openedLayerIDs.values()) {
      visibleLayerIDs.add(layerID);
    }
    return visibleLayerIDs.size;
  }

  private hitTestClientPoint(clientX: number, clientY: number): NodeID {
    const rect = this.canvas.getBoundingClientRect();
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    if (x < 0 || y < 0 || x > rect.width || y > rect.height) return '';
    this.pointer.x = (x / rect.width) * 2 - 1;
    this.pointer.y = -(y / rect.height) * 2 + 1;
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const intersects = this.raycaster.intersectObjects(this.allMeshes, false);
    for (const hit of intersects) {
      const nodeId = String(hit.object.userData?.nodeId ?? '');
      if (!nodeId) continue;
      const node = this.nodes.get(nodeId);
      if (!node || node.layerId !== this.activeLayerId || !node.mesh.visible) continue;
      return nodeId;
    }
    return '';
  }

  private selectNode(nodeId: NodeID) {
    this.selectedNodeId = nodeId;
    if (nodeId) {
      this.recentSelectedNodeIDs.unshift(nodeId);
      if (this.recentSelectedNodeIDs.length > 5) this.recentSelectedNodeIDs.length = 5;
      console.log(`[Three #three] selected node: ${nodeId}`);
      this.focusCameraOnNode(nodeId);
    }
    this.refreshVisualState();
    this.syncCanvasState();
  }

  private applyLayerView(layerId: LayerID) {
    this.activeLayerId = layerId;
    const activeLayer = this.layers.get(layerId);
    const visibleLayerIDs = new Set<LayerID>();
    visibleLayerIDs.add('root');
    visibleLayerIDs.add(layerId);
    for (const openedLayerID of this.openedLayerIDs.values()) {
      visibleLayerIDs.add(openedLayerID);
    }
    for (const layer of this.layers.values()) {
      const isVisible = visibleLayerIDs.has(layer.id);
      if (!isVisible) {
        layer.grid.visible = false;
        continue;
      }
      layer.grid.visible = true;
      const isActive = layer.id === layerId;
      const isBelowActive = !!activeLayer && layer.baseY < activeLayer.baseY - 0.001;
      if (isActive) {
        this.setLayerGridOpacity(layer, 0.6);
      } else if (isBelowActive) {
        this.setLayerGridOpacity(layer, 0.14);
      } else {
        this.setLayerGridOpacity(layer, 0.28);
      }
    }

    for (const node of this.nodes.values()) {
      const isVisible = visibleLayerIDs.has(node.layerId);
      const isActive = node.layerId === layerId;
      const nodeLayer = this.layers.get(node.layerId);
      const isBelowActive = !!activeLayer && !!nodeLayer && nodeLayer.baseY < activeLayer.baseY - 0.001;
      node.mesh.visible = isVisible;
      node.labelMesh.visible = isVisible && this.labelsVisible;
      const material = node.mesh.material as THREE.MeshStandardMaterial;
      material.transparent = !isActive;
      material.opacity = isActive ? 1 : isBelowActive ? 0.1 : 0.24;
    }
    for (const edge of this.edges.values()) {
      const isVisible = visibleLayerIDs.has(edge.layerId);
      const isActive = edge.layerId === layerId;
      const edgeLayer = this.layers.get(edge.layerId);
      const isBelowActive = !!activeLayer && !!edgeLayer && edgeLayer.baseY < activeLayer.baseY - 0.001;
      edge.line.visible = isVisible;
      const material = edge.line.material as THREE.LineBasicMaterial;
      material.transparent = !isActive;
      material.opacity = isActive ? 1 : isBelowActive ? 0.08 : 0.2;
      this.updateEdgeLine(edge);
    }
    for (const link of this.nestedLinks) {
      this.updateNestedLinkGeometry(link);
      link.line.visible = visibleLayerIDs.has(link.childLayerId);
    }
    const selectedNode = this.nodes.get(this.selectedNodeId);
    if (!selectedNode || selectedNode.layerId !== layerId || !selectedNode.mesh.visible) {
      this.fitCameraToLayer(layerId);
    } else {
      this.focusCameraOnNode(selectedNode.id);
    }
    this.refreshVisualState();
    this.syncCanvasState();
  }

  private fitCameraToLayer(layerId: LayerID) {
    this.setCameraViewForLayer(layerId, this.cameraView);
  }

  private getLayerBounds(layerId: LayerID): { ok: boolean; center: THREE.Vector3; maxDim: number } {
    const layer = this.layers.get(layerId);
    if (!layer) {
      return { ok: false, center: new THREE.Vector3(), maxDim: 6 };
    }
    if (layer.nodeIds.length === 0) {
      const center = layer.anchor.getWorldPosition(new THREE.Vector3());
      return { ok: true, center, maxDim: 6 };
    }
    const bbox = new THREE.Box3();
    for (const nodeId of layer.nodeIds) {
      const node = this.nodes.get(nodeId);
      if (!node) continue;
      bbox.expandByPoint(node.mesh.getWorldPosition(new THREE.Vector3()));
    }
    const center = bbox.getCenter(new THREE.Vector3());
    const size = bbox.getSize(new THREE.Vector3());
    return { ok: true, center, maxDim: Math.max(4, size.x, size.z) };
  }

  private positionCameraAroundPoint(center: THREE.Vector3, maxDim: number, view: CameraView) {
    this.stageCamera.framePoint(center, maxDim, view);
  }

  private getFixedNodeCameraDistance(view: CameraView): number {
    if (view === 'top') return 20;
    if (view === 'side') return 18;
    if (view === 'front') return 18;
    return 19; // iso
  }

  private focusCameraOnNode(nodeId: NodeID): boolean {
    const node = this.nodes.get(nodeId);
    if (!node) return false;
    const center = node.mesh.getWorldPosition(new THREE.Vector3());
    const fixedDistance = this.getFixedNodeCameraDistance(this.cameraView);
    this.stageCamera.framePointFixed(center, fixedDistance, this.cameraView);
    return true;
  }

  private setCameraViewForLayer(layerId: LayerID, view: CameraView): boolean {
    const bounds = this.getLayerBounds(layerId);
    if (!bounds.ok) return false;
    this.positionCameraAroundPoint(bounds.center, bounds.maxDim, view);
    return true;
  }

  private refreshVisualState() {
    const activeLayer = this.layers.get(this.activeLayerId);
    if (!activeLayer) return;

    const mostRecent = this.recentSelectedNodeIDs[0] || '';
    const secondRecent = this.recentSelectedNodeIDs[1] || '';

    for (const nodeId of activeLayer.nodeIds) {
      const node = this.nodes.get(nodeId);
      if (!node) continue;
      const material = node.mesh.material as THREE.MeshStandardMaterial;
      if (node.id === mostRecent) {
        material.color.setHex(NODE_RECENT_COLOR);
        material.emissive.setHex(0x8fb6ff);
        material.emissiveIntensity = 0.95;
      } else if (node.id === secondRecent) {
        material.color.setHex(NODE_SECOND_RECENT_COLOR);
        material.emissive.setHex(0x1f3e8e);
        material.emissiveIntensity = 0.38;
      } else {
        material.color.setHex(NODE_BASE_COLOR);
        material.emissive.setHex(0x000000);
        material.emissiveIntensity = 0.1;
      }
    }

    for (const edgeId of activeLayer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge) continue;
      const material = edge.line.material as THREE.LineBasicMaterial;
      material.color.setHex(EDGE_BASE_COLOR);
    }
  }

  private syncCanvasState() {
    this.canvas.setAttribute('data-active-layer', this.activeLayerId);
    this.canvas.setAttribute('data-selected-node', this.selectedNodeId);
    this.canvas.setAttribute('data-history-depth', String(this.history.length));
    this.syncControlState();
  }

  private getVisibleNodeIDs(): NodeID[] {
    const layer = this.layers.get(this.activeLayerId);
    return layer ? [...layer.nodeIds] : [];
  }

  private getInputNodeIDs(nodeId: NodeID): NodeID[] {
    if (!nodeId) return [];
    const layer = this.layers.get(this.activeLayerId);
    if (!layer) return [];
    const out: NodeID[] = [];
    for (const edgeId of layer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge) continue;
      if (edge.inputNodeId === nodeId) out.push(edge.outputNodeId);
    }
    return out.sort();
  }

  private getOutputNodeIDs(nodeId: NodeID): NodeID[] {
    if (!nodeId) return [];
    const layer = this.layers.get(this.activeLayerId);
    if (!layer) return [];
    const out: NodeID[] = [];
    for (const edgeId of layer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge) continue;
      if (edge.outputNodeId === nodeId) out.push(edge.inputNodeId);
    }
    return out.sort();
  }

  private getNestedNodeIDs(nodeId: NodeID): NodeID[] {
    const node = this.nodes.get(nodeId);
    if (!node || node.nestedLayerIDs.length === 0) return [];
    const out = new Set<NodeID>();
    for (const layerID of node.nestedLayerIDs) {
      const layer = this.layers.get(layerID);
      if (!layer) continue;
      for (const nestedNodeID of layer.nodeIds) out.add(nestedNodeID);
    }
    return [...out].sort();
  }

  private getProjectedPoint(nodeId: NodeID): ProjectedPoint {
    const node = this.nodes.get(nodeId);
    if (!node || !node.mesh.visible) return { ok: false, x: 0, y: 0 };
    const rect = this.canvas.getBoundingClientRect();
    this.scene.updateMatrixWorld(true);
    this.camera.updateMatrixWorld(true);
    const world = node.mesh.getWorldPosition(new THREE.Vector3());
    const projected = world.project(this.camera);
    const x = Math.round((projected.x * 0.5 + 0.5) * rect.width + rect.left);
    const y = Math.round((-projected.y * 0.5 + 0.5) * rect.height + rect.top);
    return { ok: true, x, y };
  }

  private clickProjected(nodeId: NodeID): boolean {
    const p = this.getProjectedPoint(nodeId);
    if (!p.ok) return false;
    this.canvas.dispatchEvent(
      new MouseEvent('click', {
        clientX: p.x,
        clientY: p.y,
        button: 0,
        bubbles: true,
        cancelable: true,
        view: window,
      })
    );
    return this.selectedNodeId === nodeId;
  }

  private popSelectionHistoryBack(): boolean {
    if (this.recentSelectedNodeIDs.length < 2) return false;
    this.recentSelectedNodeIDs.shift();
    const prevNodeId = this.recentSelectedNodeIDs[0];
    if (!prevNodeId) return false;
    const prevNode = this.nodes.get(prevNodeId);
    if (!prevNode) {
      this.recentSelectedNodeIDs = this.recentSelectedNodeIDs.filter((id) => this.nodes.has(id));
      this.selectedNodeId = this.recentSelectedNodeIDs[0] || '';
      this.refreshVisualState();
      this.syncCanvasState();
      return this.selectedNodeId !== '';
    }
    this.selectedNodeId = prevNodeId;
    this.applyLayerView(prevNode.layerId);
    console.log(`[Three #three] back to selected node: ${prevNodeId}`);
    return true;
  }

  private openNestedLayer(nodeId?: NodeID): boolean {
    const targetNode = this.nodes.get(nodeId || this.selectedNodeId);
    if (!targetNode || targetNode.nestedLayerIDs.length === 0) return false;
    const candidateLayerIDs = targetNode.nestedLayerIDs.filter((layerID) => this.layers.has(layerID));
    if (candidateLayerIDs.length === 0) return false;
    const targetLayerID = candidateLayerIDs[candidateLayerIDs.length - 1];
    this.openedLayerIDs.add(targetLayerID);
    this.history.push({ layerId: this.activeLayerId, selectedNodeId: this.selectedNodeId });
    this.selectedNodeId = '';
    this.applyLayerView(targetLayerID);
    console.log(`[Three #three] open nested layer: ${targetLayerID}`);
    return true;
  }

  private clearSelectionHistoryForLayer(layerId: LayerID) {
    this.recentSelectedNodeIDs = this.recentSelectedNodeIDs.filter((nodeId) => {
      const node = this.nodes.get(nodeId);
      return !!node && node.layerId !== layerId;
    });
  }

  private navigateBackToParentLayer(): boolean {
    const prev = this.history.pop();
    if (!prev) return false;
    this.selectedNodeId = prev.selectedNodeId || '';
    this.applyLayerView(prev.layerId);
    console.log(`[Three #three] back layer: ${prev.layerId}`);
    return true;
  }

  private closeNestedLayerByID(layerId: LayerID): boolean {
    if (!this.openedLayerIDs.has(layerId)) return false;
    this.openedLayerIDs.delete(layerId);
    this.clearSelectionHistoryForLayer(layerId);

    if (this.activeLayerId === layerId) {
      const layer = this.layers.get(layerId);
      const parentNodeID = layer?.parentNodeId || '';
      const parentNode = parentNodeID ? this.nodes.get(parentNodeID) : null;
      const targetLayerID = parentNode?.layerId || 'root';
      this.history = this.history.filter((snap) => snap.layerId !== layerId);
      this.selectedNodeId = parentNodeID;
      this.applyLayerView(targetLayerID);
    } else {
      this.applyLayerView(this.activeLayerId);
    }
    console.log(`[Three #three] close nested layer: ${layerId}`);
    return true;
  }

  private addNode(layerId: LayerID, nodeId: NodeID, rank: number, row: number): boolean {
    if (this.nodes.has(nodeId) || !this.layers.has(layerId)) return false;
    const finalRow = this.findAvailableRow(layerId, rank, row);
    this.createNode(nodeId, layerId, rank, finalRow);
    this.normalizeLayerLayout(layerId);
    this.applyLayerView(this.activeLayerId);
    return true;
  }

  private addEdge(edgeId: EdgeID, layerId: LayerID, outputNodeId: NodeID, inputNodeId: NodeID): boolean {
    if (this.edges.has(edgeId)) return false;
    const outNode = this.nodes.get(outputNodeId);
    const inNode = this.nodes.get(inputNodeId);
    if (!outNode || !inNode) return false;
    if (outNode.layerId !== layerId || inNode.layerId !== layerId) return false;
    this.createEdge(edgeId, layerId, outputNodeId, inputNodeId);
    this.reconcileInputNodeRank(layerId, inputNodeId);
    this.normalizeLayerLayout(layerId);
    this.applyLayerView(this.activeLayerId);
    return true;
  }

  private findAvailableRow(layerId: LayerID, rank: number, desiredRow: number): number {
    const layer = this.layers.get(layerId);
    if (!layer) return desiredRow;
    let row = Math.max(0, desiredRow);
    for (;;) {
      const occupied = layer.nodeIds.some((id) => {
        const n = this.nodes.get(id);
        return !!n && n.rank === rank && n.row === row;
      });
      if (!occupied) return row;
      row += 1;
    }
  }

  private updateNodePosition(node: DagNode) {
    node.mesh.position.set(this.rankToX(node.rank), 0, this.rowToZ(node.row));
    node.labelMesh.position.set(this.rankToX(node.rank) + 2.25, 0, this.rowToZ(node.row));
    for (const edge of this.edges.values()) {
      if (edge.inputNodeId === node.id || edge.outputNodeId === node.id) {
        this.updateEdgeLine(edge);
      }
    }
    for (const link of this.nestedLinks) {
      if (link.parentNodeId === node.id || link.childNodeId === node.id) {
        this.updateNestedLinkGeometry(link);
      }
    }
  }

  private reconcileInputNodeRank(layerId: LayerID, inputNodeId: NodeID) {
    const inputNode = this.nodes.get(inputNodeId);
    const layer = this.layers.get(layerId);
    if (!inputNode || !layer) return;
    let maxOutputRank = -1;
    for (const edgeId of layer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge || edge.inputNodeId !== inputNodeId) continue;
      const outputNode = this.nodes.get(edge.outputNodeId);
      if (!outputNode) continue;
      if (outputNode.rank > maxOutputRank) maxOutputRank = outputNode.rank;
    }
    if (maxOutputRank < 0) return;
    const targetRank = maxOutputRank + 1;
    if (inputNode.rank < targetRank) {
      inputNode.rank = targetRank;
      inputNode.row = this.findAvailableRow(layerId, targetRank, inputNode.row);
      this.updateNodePosition(inputNode);
    }
  }

  private createNodeAtClient(layerId: LayerID, clientX: number, clientY: number): NodeID {
    const layer = this.layers.get(layerId);
    if (!layer) return '';
    const rect = this.canvas.getBoundingClientRect();
    const x = clientX - rect.left;
    const y = clientY - rect.top;
    if (x < 0 || y < 0 || x > rect.width || y > rect.height) return '';

    this.pointer.x = (x / rect.width) * 2 - 1;
    this.pointer.y = -(y / rect.height) * 2 + 1;
    this.raycaster.setFromCamera(this.pointer, this.camera);
    const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -layer.baseY);
    const world = new THREE.Vector3();
    if (!this.raycaster.ray.intersectPlane(plane, world)) return '';

    const approxRank = Math.max(0, Math.round((world.x + this.rankXSpacing) / this.rankXSpacing));
    const approxRow = Math.max(0, Math.round((layer.baseZ - world.z) / this.rowZSpacing));
    const finalRow = this.findAvailableRow(layerId, approxRank, approxRow);
    const nodeId = `n_user_${this.userNodeCounter++}`;
    this.createNode(nodeId, layerId, approxRank, finalRow);
    this.applyLayerView(this.activeLayerId);
    return nodeId;
  }

  private removeEdge(edgeId: EdgeID): boolean {
    const edge = this.edges.get(edgeId);
    if (!edge) return false;
    this.edges.delete(edgeId);
    const layer = this.layers.get(edge.layerId);
    if (layer) layer.edgeIds = layer.edgeIds.filter((id) => id !== edgeId);
    layer?.anchor.remove(edge.line);
    edge.line.geometry.dispose();
    (edge.line.material as THREE.Material).dispose();
    this.normalizeLayerLayout(edge.layerId);
    this.applyLayerView(this.activeLayerId);
    return true;
  }

  private removeNode(nodeId: NodeID): boolean {
    const node = this.nodes.get(nodeId);
    if (!node) return false;
    const layer = this.layers.get(node.layerId);
    if (!layer) return false;

    const deleteEdgeIDs: EdgeID[] = [];
    for (const edge of this.edges.values()) {
      if (edge.inputNodeId === nodeId || edge.outputNodeId === nodeId) deleteEdgeIDs.push(edge.id);
    }
    for (const edgeId of deleteEdgeIDs) this.removeEdge(edgeId);

    if (node.nestedLayerIDs.length > 0) {
      for (const nestedLayerID of node.nestedLayerIDs) {
        const nestedLayer = this.layers.get(nestedLayerID);
        if (!nestedLayer) continue;
        for (const nestedNodeID of [...nestedLayer.nodeIds]) this.removeNode(nestedNodeID);
        for (const nestedEdgeID of [...nestedLayer.edgeIds]) this.removeEdge(nestedEdgeID);
        this.layers.delete(nestedLayerID);
      }
      node.nestedLayerIDs = [];
    }

    const keptNestedLinks: NestedLink[] = [];
    for (const link of this.nestedLinks) {
      if (link.parentNodeId === nodeId || link.childNodeId === nodeId) {
        this.scene.remove(link.line);
        link.line.geometry.dispose();
        (link.line.material as THREE.Material).dispose();
      } else {
        keptNestedLinks.push(link);
      }
    }
    this.nestedLinks = keptNestedLinks;

    layer.anchor.remove(node.mesh);
    layer.anchor.remove(node.labelMesh);
    node.mesh.geometry.dispose();
    (node.mesh.material as THREE.Material).dispose();
    node.labelMesh.geometry.dispose();
    const labelMaterial = node.labelMesh.material as THREE.MeshBasicMaterial;
    if (labelMaterial.map) labelMaterial.map.dispose();
    labelMaterial.dispose();
    this.nodes.delete(nodeId);
    this.allMeshes = this.allMeshes.filter((m) => m !== node.mesh);
    layer.nodeIds = layer.nodeIds.filter((id) => id !== nodeId);
    if (this.selectedNodeId === nodeId) this.selectedNodeId = '';
    this.recentSelectedNodeIDs = this.recentSelectedNodeIDs.filter((id) => id !== nodeId);
    this.normalizeLayerLayout(node.layerId);
    this.applyLayerView(this.activeLayerId);
    return true;
  }

  private getState() {
    return {
      activeLayerId: this.activeLayerId,
      selectedNodeId: this.selectedNodeId,
      visibleNodeIDs: this.getVisibleNodeIDs(),
      inputNodeIDs: this.getInputNodeIDs(this.selectedNodeId),
      outputNodeIDs: this.getOutputNodeIDs(this.selectedNodeId),
      nestedNodeIDs: this.getNestedNodeIDs(this.selectedNodeId),
      historyDepth: this.history.length,
      labelsVisible: this.labelsVisible,
      recentSelectedNodeIDs: [...this.recentSelectedNodeIDs],
      lastCreatedNodeId: this.lastCreatedNodeId,
      selectedNodeLabel: this.selectedNodeId ? (this.nodes.get(this.selectedNodeId)?.label ?? '') : '',
      camera: { x: this.camera.position.x, y: this.camera.position.y, z: this.camera.position.z },
    };
  }

  private getNodeTransform(nodeId: NodeID) {
    const node = this.nodes.get(nodeId);
    if (!node) {
      return { ok: false, id: nodeId, layerId: '', rank: -1, row: -1, position: { x: 0, y: 0, z: 0 }, quaternion: { x: 0, y: 0, z: 0, w: 1 } };
    }
    const p = node.mesh.getWorldPosition(new THREE.Vector3());
    const q = node.mesh.getWorldQuaternion(new THREE.Quaternion());
    return {
      ok: true,
      id: node.id,
      layerId: node.layerId,
      rank: node.rank,
      row: node.row,
      position: { x: p.x, y: p.y, z: p.z },
      quaternion: { x: q.x, y: q.y, z: q.z, w: q.w },
    };
  }

  private getLayerTransform(layerId: LayerID) {
    const layer = this.layers.get(layerId);
    if (!layer) {
      return { ok: false, id: layerId, baseX: 0, baseY: 0, baseZ: 0, position: { x: 0, y: 0, z: 0 }, quaternion: { x: 0, y: 0, z: 0, w: 1 } };
    }
    const p = layer.anchor.getWorldPosition(new THREE.Vector3());
    const q = layer.anchor.getWorldQuaternion(new THREE.Quaternion());
    return {
      ok: true,
      id: layer.id,
      baseX: layer.baseX,
      baseY: layer.baseY,
      baseZ: layer.baseZ,
      position: { x: p.x, y: p.y, z: p.z },
      quaternion: { x: q.x, y: q.y, z: q.z, w: q.w },
    };
  }

  private getCameraTransform() {
    const q = this.camera.quaternion;
    return {
      position: { x: this.camera.position.x, y: this.camera.position.y, z: this.camera.position.z },
      quaternion: { x: q.x, y: q.y, z: q.z, w: q.w },
    };
  }

  private setCameraView(view: CameraView): boolean {
    this.cameraView = view;
    const selected = this.nodes.get(this.selectedNodeId);
    const ok = selected && selected.layerId === this.activeLayerId ? this.focusCameraOnNode(selected.id) : this.setCameraViewForLayer(this.activeLayerId, view);
    this.syncControlState();
    return ok;
  }

  private logLayoutSnapshot(layerId: LayerID, nodeIDs: NodeID[]) {
    const layer = this.getLayerTransform(layerId);
    const camera = this.getCameraTransform();
    console.log(
      `[Three #three] layout layer=${layerId} layer_pos=(${layer.position.x.toFixed(2)},${layer.position.y.toFixed(2)},${layer.position.z.toFixed(2)}) layer_quat=(${layer.quaternion.x.toFixed(3)},${layer.quaternion.y.toFixed(3)},${layer.quaternion.z.toFixed(3)},${layer.quaternion.w.toFixed(3)}) camera_pos=(${camera.position.x.toFixed(2)},${camera.position.y.toFixed(2)},${camera.position.z.toFixed(2)}) camera_quat=(${camera.quaternion.x.toFixed(3)},${camera.quaternion.y.toFixed(3)},${camera.quaternion.z.toFixed(3)},${camera.quaternion.w.toFixed(3)})`
    );
    for (const nodeId of nodeIDs) {
      const n = this.getNodeTransform(nodeId);
      if (!n.ok) continue;
      console.log(
        `[Three #three] layout node=${n.id} layer=${n.layerId} rank=${n.rank} row=${n.row} pos=(${n.position.x.toFixed(2)},${n.position.y.toFixed(2)},${n.position.z.toFixed(2)}) quat=(${n.quaternion.x.toFixed(3)},${n.quaternion.y.toFixed(3)},${n.quaternion.z.toFixed(3)},${n.quaternion.w.toFixed(3)})`
      );
    }
  }

  private normalizeLayerLayout(layerId: LayerID) {
    const layer = this.layers.get(layerId);
    if (!layer) return;

    const byRank = new Map<number, DagNode[]>();
    for (const nodeId of layer.nodeIds) {
      const node = this.nodes.get(nodeId);
      if (!node) continue;
      if (!byRank.has(node.rank)) byRank.set(node.rank, []);
      byRank.get(node.rank)?.push(node);
    }

    for (const [rank, nodes] of byRank.entries()) {
      nodes.sort((a, b) => {
        if (a.row !== b.row) return a.row - b.row;
        return a.id.localeCompare(b.id);
      });
      for (let i = 0; i < nodes.length; i += 1) {
        const node = nodes[i];
        node.rank = rank;
        node.row = i;
        this.updateNodePosition(node);
      }
    }
  }

  private getNodeWorldPosition(nodeId: NodeID): { ok: boolean; x: number; y: number; z: number } {
    const node = this.nodes.get(nodeId);
    if (!node) return { ok: false, x: 0, y: 0, z: 0 };
    const p = node.mesh.getWorldPosition(new THREE.Vector3());
    return { ok: true, x: p.x, y: p.y, z: p.z };
  }

  private getNodeColorHex(nodeId: NodeID): { ok: boolean; colorHex: number } {
    const node = this.nodes.get(nodeId);
    if (!node) return { ok: false, colorHex: 0 };
    const material = node.mesh.material as THREE.MeshStandardMaterial;
    return { ok: true, colorHex: material.color.getHex() };
  }

  private attachDebugBridge() {
    (window as Window & { dagHitTestDebug?: Record<string, unknown> }).dagHitTestDebug = {
      getState: () => this.getState(),
      getProjectedPoint: (nodeId: NodeID) => this.getProjectedPoint(nodeId),
      getNodeWorldPosition: (nodeId: NodeID) => this.getNodeWorldPosition(nodeId),
      getNodeColorHex: (nodeId: NodeID) => this.getNodeColorHex(nodeId),
      getNodeLabel: (nodeId: NodeID) => this.nodes.get(nodeId)?.label ?? '',
      getNodeTransform: (nodeId: NodeID) => this.getNodeTransform(nodeId),
      getLayerTransform: (layerId: LayerID) => this.getLayerTransform(layerId),
      getCameraTransform: () => this.getCameraTransform(),
      setCameraView: (view: CameraView) => this.setCameraView(view),
      appendThought: (text: string) => this.appendThought(text),
      logLayoutSnapshot: (layerId: LayerID, nodeIDs: NodeID[]) => this.logLayoutSnapshot(layerId, nodeIDs),
      clickProjected: (nodeId: NodeID) => this.clickProjected(nodeId),
      createNodeAtClient: (layerId: LayerID, clientX: number, clientY: number) => this.createNodeAtClient(layerId, clientX, clientY),
      openNestedLayer: (nodeId?: NodeID) => this.openNestedLayer(nodeId),
      closeActiveLayer: () => this.performOpenOrCloseLayerAction(),
      enterNested: (nodeId?: NodeID) => this.openNestedLayer(nodeId),
      goBack: () => this.performBackAction(),
      addNode: (layerId: LayerID, nodeId: NodeID, rank: number, row: number) => this.addNode(layerId, nodeId, rank, row),
      addEdge: (edgeId: EdgeID, layerId: LayerID, outputNodeId: NodeID, inputNodeId: NodeID) =>
        this.addEdge(edgeId, layerId, outputNodeId, inputNodeId),
      removeNode: (nodeId: NodeID) => this.removeNode(nodeId),
      removeEdge: (edgeId: EdgeID) => this.removeEdge(edgeId),
    };
  }

  private resize = () => {
    const rect = this.container.getBoundingClientRect();
    const width = Math.max(1, rect.width);
    const height = Math.max(1, rect.height);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  private animate = () => {
    this.frameID = requestAnimationFrame(this.animate);
    if (!this.visible) return;

    const width = Math.max(1, this.container.clientWidth);
    const height = Math.max(1, this.container.clientHeight);

    this.renderer.setViewport(0, 0, width, height);
    this.renderer.setScissorTest(false);
    this.renderer.clear(true, true, true);
    this.renderer.render(this.scene, this.camera);

    const gizmoSize = Math.max(76, Math.round(Math.min(width, height) * 0.14));
    const pad = 12;
    this.gizmoAxes.quaternion.copy(this.camera.quaternion).invert();
    this.renderer.clearDepth();
    this.renderer.setScissor(pad, pad, gizmoSize, gizmoSize);
    this.renderer.setViewport(pad, pad, gizmoSize, gizmoSize);
    this.renderer.setScissorTest(true);
    this.renderer.render(this.gizmoScene, this.gizmoCamera);
    this.renderer.setScissorTest(false);
  };

  dispose(): void {
    cancelAnimationFrame(this.frameID);
    window.removeEventListener('resize', this.resize);
    this.canvas.removeEventListener('click', this.onClick);
    this.canvas.removeEventListener('dblclick', this.onDoubleClick);
    const win = window as Window & { dagHitTestDebug?: unknown };
    if (win.dagHitTestDebug) delete win.dagHitTestDebug;
    this.chatlogTerm?.dispose();
    this.chatlogTerm = null;
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (!visible) return;
    // When section visibility changes (especially after refresh/hash restore),
    // container dimensions can differ from initialization. Re-sync camera/renderer.
    this.resize();
    requestAnimationFrame(() => this.resize());
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  return new ThreeControl(container, canvas);
}
