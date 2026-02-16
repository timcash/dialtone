import * as THREE from 'three';
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';

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
  nestedLayerId?: LayerID;
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
  nodeIds: NodeID[];
  edgeIds: EdgeID[];
};

type LayerSnapshot = {
  layerId: LayerID;
  selectedNodeId: NodeID;
};

type ProjectedPoint = { ok: boolean; x: number; y: number };
type CameraView = 'iso' | 'top' | 'side' | 'front';
type ActionMode = 'add' | 'connect' | 'nest' | 'enter' | 'remove-node' | 'remove-edge' | 'labels';
type NestedLink = {
  parentNodeId: NodeID;
  childNodeId: NodeID;
  childLayerId: LayerID;
  line: THREE.Line;
};

const NODE_BASE_COLOR = 0x5b6873;
const NODE_SELECTED_COLOR = 0x2b78ff;
const NODE_INPUT_COLOR = 0x2da44e;
const NODE_OUTPUT_COLOR = 0xd97706;
const NODE_NESTED_COLOR = 0x7c3aed;
const NODE_NESTED_LAYER_COLOR = 0x2f9e9f;
const EDGE_BASE_COLOR = 0x6b7280;
const EDGE_NESTED_LINK_COLOR = 0xfacc15;
const MODE_SEQUENCE: ActionMode[] = ['add', 'connect', 'nest', 'enter', 'remove-node', 'remove-edge', 'labels'];

class ThreeControl implements VisualizationControl {
  private scene = new THREE.Scene();
  private camera = new THREE.PerspectiveCamera(50, 1, 0.1, 400);
  private renderer: THREE.WebGLRenderer;
  private gizmoScene = new THREE.Scene();
  private gizmoCamera = new THREE.PerspectiveCamera(50, 1, 0.1, 10);
  private gizmoAxes = new THREE.AxesHelper(0.8);
  private raycaster = new THREE.Raycaster();
  private pointer = new THREE.Vector2(2, 2);
  private frameID = 0;
  private visible = false;

  private rankXSpacing = 8;
  private rowYSpacing = 5.5;
  private nestedLayerZOffset = 15;

  private layers = new Map<LayerID, DagLayer>();
  private nodes = new Map<NodeID, DagNode>();
  private edges = new Map<EdgeID, DagEdge>();
  private selectedNodeId = '';
  private activeLayerId: LayerID = 'root';
  private history: LayerSnapshot[] = [];
  private allMeshes: THREE.Mesh[] = [];
  private keyLight: THREE.DirectionalLight;
  private nestedLinks: NestedLink[] = [];

  private userNodeCounter = 1;
  private edgeCounter = 1;
  private mode: ActionMode = 'add';
  private pendingConnectSourceNodeId = '';
  private labelsVisible = false;
  private lastCreatedNodeId = '';
  private menuOpen = false;
  private lastTapAtByControl = new Map<string, number>();
  private renameInput: HTMLInputElement | null = null;
  private renameApplyButton: HTMLButtonElement | null = null;

  private backButton: HTMLButtonElement | null = null;
  private actionButton: HTMLButtonElement | null = null;
  private modeButton: HTMLButtonElement | null = null;
  private menuButton: HTMLButtonElement | null = null;
  private menuPanel: HTMLElement | null = null;
  private backStack: HTMLElement | null = null;
  private actionStack: HTMLElement | null = null;
  private modeStack: HTMLElement | null = null;
  private menuTableButton: HTMLButtonElement | null = null;
  private menuThreeButton: HTMLButtonElement | null = null;
  private actionLabelsButton: HTMLButtonElement | null = null;

  constructor(private container: HTMLElement, private canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas, antialias: true });
    this.renderer.setClearColor(0x05070a, 1);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.autoClear = false;

    this.camera.position.set(0, 0, 28);
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
    this.attachLayerGrid(anchor);
    this.scene.add(anchor);
    this.layers.set(id, { id, parentNodeId, baseX, baseY, baseZ, anchor, nodeIds: [], edgeIds: [] });
  }

  private attachLayerGrid(anchor: THREE.Group) {
    const width = 40;
    const height = 28;
    const y0 = -2.75;
    const z = -1.1;
    const lineColor = 0xb9c8df;

    // Sparse dashed guides, no filled background.
    const xGuides = [-width / 3, 0, width / 3];
    const yGuides = [-height / 4, 0, height / 4];

    for (const gx of xGuides) {
      const line = this.createDashedLine(
        new THREE.Vector3(gx, y0 - height / 2, z),
        new THREE.Vector3(gx, y0 + height / 2, z),
        lineColor
      );
      anchor.add(line);
    }
    for (const gy of yGuides) {
      const line = this.createDashedLine(
        new THREE.Vector3(-width / 2, y0 + gy, z),
        new THREE.Vector3(width / 2, y0 + gy, z),
        lineColor
      );
      anchor.add(line);
    }
  }

  private createDashedLine(from: THREE.Vector3, to: THREE.Vector3, color: number): THREE.Line {
    const geometry = new THREE.BufferGeometry().setFromPoints([from, to]);
    const material = new THREE.LineDashedMaterial({
      color,
      transparent: true,
      opacity: 0.24,
      dashSize: 0.8,
      gapSize: 0.7,
      depthWrite: false,
    });
    const line = new THREE.Line(geometry, material);
    line.computeLineDistances();
    return line;
  }

  private rankToX(rank: number): number {
    return rank * this.rankXSpacing - this.rankXSpacing;
  }

  private rowToY(row: number): number {
    return -row * this.rowYSpacing;
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

  private createNode(id: NodeID, layerId: LayerID, rank: number, row: number, nestedLayerId?: LayerID) {
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
    mesh.position.set(this.rankToX(rank), this.rowToY(row), 0);
    mesh.userData = { nodeId: id };
    layer.anchor.add(mesh);
    const label = id;
    const labelAsset = this.createLabelMesh(label);
    const labelMesh = labelAsset.mesh;
    labelMesh.position.set(this.rankToX(rank) + 2.25, this.rowToY(row), 0);
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
      nestedLayerId,
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
    if (parentNode.nestedLayerId && this.layers.has(parentNode.nestedLayerId)) {
      return parentNode.nestedLayerId;
    }

    const parentLayer = this.layers.get(parentNode.layerId);
    const parentWorld = parentNode.mesh.getWorldPosition(new THREE.Vector3());
    const parentBaseZ = parentLayer ? parentLayer.baseZ : 0;
    const nestedLayerId = `layer_nested_${nodeId}`;
    this.createLayer(nestedLayerId, nodeId, parentWorld.x, parentWorld.y, parentBaseZ - this.nestedLayerZOffset);
    parentNode.nestedLayerId = nestedLayerId;
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
    this.enterNested(this.selectedNodeId);
  };

  private initMobileUI() {
    this.backButton = this.container.querySelector("button[aria-label='DAG Back']");
    this.actionButton = this.container.querySelector("button[aria-label='DAG Action']");
    this.modeButton = this.container.querySelector("button[aria-label='DAG Mode']");
    this.menuButton = this.container.querySelector("button[aria-label='DAG Menu']");
    this.menuPanel = this.container.querySelector("[aria-label='DAG Menu Panel']");
    this.backStack = this.container.querySelector("[aria-label='DAG Back Stack']");
    this.actionStack = this.container.querySelector("[aria-label='DAG Action Stack']");
    this.modeStack = this.container.querySelector("[aria-label='DAG Mode Stack']");
    this.menuTableButton = this.container.querySelector("button[aria-label='DAG Menu Navigate Table']");
    this.menuThreeButton = this.container.querySelector("button[aria-label='DAG Menu Navigate Three']");
    this.actionLabelsButton = this.container.querySelector("button[aria-label='DAG Action Labels']");
    this.renameInput = this.container.querySelector("input[aria-label='DAG Node Name']");
    this.renameApplyButton = this.container.querySelector("button[aria-label='DAG Rename Node']");
    const testMode = (() => {
      try {
        return window.sessionStorage.getItem('dag_test_mode') === '1';
      } catch {
        return false;
      }
    })();
    const backRootButton = this.container.querySelector("button[aria-label='DAG Back Root']");
    const actionNestButton = this.container.querySelector("button[aria-label='DAG Action Nest']");
    const actionClearButton = this.container.querySelector("button[aria-label='DAG Action Clear']");
    const modePrevButton = this.container.querySelector("button[aria-label='DAG Mode Prev']");

    this.bindThumbWithDoubleTap('back', this.backButton, this.backStack, () => {
      this.goBack();
      this.syncControlState();
    });
    this.bindThumbWithDoubleTap('action', this.actionButton, this.actionStack, () => {
      this.performAction();
      this.syncControlState();
    });
    this.bindThumbWithDoubleTap('mode', this.modeButton, this.modeStack, () => {
      this.cycleMode(1);
      this.syncControlState();
    });

    backRootButton?.addEventListener('click', () => {
      this.goBackToRoot();
      this.collapseStacks();
      this.syncControlState();
    });
    actionClearButton?.addEventListener('click', () => {
      this.pendingConnectSourceNodeId = '';
      this.collapseStacks();
      this.syncControlState();
    });
    actionNestButton?.addEventListener('click', () => {
      this.createNestedLayerAndEnter();
      this.collapseStacks();
      this.syncControlState();
    });
    modePrevButton?.addEventListener('click', () => {
      this.cycleMode(-1);
      this.collapseStacks();
      this.syncControlState();
    });

    this.menuButton?.addEventListener('click', () => {
      if (testMode) {
        this.menuOpen = false;
        if (this.menuPanel) this.menuPanel.hidden = true;
        this.syncControlState();
        return;
      }
      this.menuOpen = !this.menuOpen;
      this.syncControlState();
    });
    this.menuTableButton?.addEventListener('click', () => {
      const sections = (window as Window & { sections?: { navigateTo: (id: string) => Promise<void> } }).sections;
      void sections?.navigateTo('dag-table');
      this.menuOpen = false;
      this.syncControlState();
    });
    this.menuThreeButton?.addEventListener('click', () => {
      const sections = (window as Window & { sections?: { navigateTo: (id: string) => Promise<void> } }).sections;
      void sections?.navigateTo('three');
      this.menuOpen = false;
      this.syncControlState();
    });
    this.actionLabelsButton?.addEventListener('click', () => {
      this.toggleLabels();
      this.collapseStacks();
      this.syncControlState();
    });
    this.renameApplyButton?.addEventListener('click', () => this.applyRenameFromInput());
    this.renameInput?.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') return;
      event.preventDefault();
      this.applyRenameFromInput();
    });
    if (testMode) {
      if (this.menuButton) this.menuButton.setAttribute('data-test-locked', 'true');
      if (this.menuPanel) this.menuPanel.hidden = true;
      this.menuOpen = false;
    }

    this.syncControlState();
  }

  private bindThumbWithDoubleTap(
    controlID: string,
    button: HTMLButtonElement | null,
    stack: HTMLElement | null,
    onSingleTap: () => void
  ) {
    if (!button) return;
    const onTap = () => {
      onSingleTap();
      const now = Date.now();
      const prev = this.lastTapAtByControl.get(controlID) ?? 0;
      if (now - prev <= 280 && stack) {
        stack.hidden = !stack.hidden;
      }
      this.lastTapAtByControl.set(controlID, now);
      this.syncControlState();
    };
    button.addEventListener('click', onTap);
    button.addEventListener('dblclick', () => {
      if (!stack) return;
      stack.hidden = !stack.hidden;
      this.syncControlState();
    });
  }

  private collapseStacks() {
    if (this.backStack) this.backStack.hidden = true;
    if (this.actionStack) this.actionStack.hidden = true;
    if (this.modeStack) this.modeStack.hidden = true;
  }

  private cycleMode(delta: -1 | 1) {
    const idx = MODE_SEQUENCE.indexOf(this.mode);
    const nextIdx = (idx + delta + MODE_SEQUENCE.length) % MODE_SEQUENCE.length;
    this.mode = MODE_SEQUENCE[nextIdx];
    if (this.mode !== 'connect') this.pendingConnectSourceNodeId = '';
  }

  private modeToLabel(mode: ActionMode): string {
    switch (mode) {
      case 'add':
        return 'Add';
      case 'connect':
        return 'Link';
      case 'nest':
        return 'Nest';
      case 'enter':
        return 'Dive';
      case 'remove-node':
        return 'DelN';
      case 'remove-edge':
        return 'DelE';
      case 'labels':
        return 'Label';
      default:
        return 'Mode';
    }
  }

  private performAction() {
    switch (this.mode) {
      case 'add':
        this.createChildNodeFromSelected();
        return;
      case 'connect':
        this.performConnectAction();
        return;
      case 'nest':
        this.createNestedLayerAndEnter();
        return;
      case 'enter':
        if (this.selectedNodeId) this.enterNested(this.selectedNodeId);
        return;
      case 'remove-node':
        this.performRemoveNodeAction();
        return;
      case 'remove-edge':
        this.performRemoveEdgeAction();
        return;
      case 'labels':
        this.toggleLabels();
        return;
      default:
        return;
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
    const edgeId = `e_user_${this.edgeCounter++}`;
    this.addEdge(edgeId, source.layerId, source.id, nodeId);
    this.lastCreatedNodeId = nodeId;
    this.selectNode(nodeId);
    console.log(`[Three #three] action add node: ${nodeId} from ${source.id}`);
  }

  private createNestedLayerAndEnter() {
    if (!this.selectedNodeId) return;
    const nestedLayerId = this.ensureNestedLayer(this.selectedNodeId);
    if (!nestedLayerId) return;
    this.enterNested(this.selectedNodeId);
  }

  private performConnectAction() {
    if (!this.selectedNodeId) return;
    if (!this.pendingConnectSourceNodeId) {
      this.pendingConnectSourceNodeId = this.selectedNodeId;
      return;
    }
    if (this.pendingConnectSourceNodeId === this.selectedNodeId) return;
    const source = this.nodes.get(this.pendingConnectSourceNodeId);
    const target = this.nodes.get(this.selectedNodeId);
    if (!source || !target || source.layerId !== target.layerId) return;
    const edgeId = `e_user_${this.edgeCounter++}`;
    if (this.addEdge(edgeId, source.layerId, source.id, target.id)) {
      console.log(`[Three #three] action add edge: ${source.id} -> ${target.id}`);
    }
    this.pendingConnectSourceNodeId = '';
  }

  private performRemoveNodeAction() {
    if (!this.selectedNodeId) return;
    const nodeID = this.selectedNodeId;
    if (this.removeNode(nodeID)) {
      console.log(`[Three #three] action remove node: ${nodeID}`);
    }
  }

  private performRemoveEdgeAction() {
    if (!this.selectedNodeId) return;
    const stateEdges: EdgeID[] = [];
    const layer = this.layers.get(this.activeLayerId);
    if (!layer) return;
    for (const edgeId of layer.edgeIds) {
      const edge = this.edges.get(edgeId);
      if (!edge) continue;
      if (edge.inputNodeId === this.selectedNodeId || edge.outputNodeId === this.selectedNodeId) stateEdges.push(edge.id);
    }
    const edgeID = stateEdges.sort()[0];
    if (!edgeID) return;
    if (this.removeEdge(edgeID)) {
      console.log(`[Three #three] action remove edge: ${edgeID}`);
    }
  }

  private toggleLabels() {
    this.labelsVisible = !this.labelsVisible;
    for (const node of this.nodes.values()) {
      node.labelMesh.visible = node.mesh.visible && this.labelsVisible;
    }
    this.syncControlState();
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

  private goBackToRoot() {
    while (this.goBack()) {
      // Keep popping history until root.
    }
  }

  private syncControlState() {
    if (this.modeButton) this.modeButton.textContent = this.modeToLabel(this.mode);
    if (this.actionButton) {
      if (this.mode === 'connect' && this.pendingConnectSourceNodeId) {
        this.actionButton.textContent = 'Link+';
      } else if (this.mode === 'nest') {
        this.actionButton.textContent = 'Nest';
      } else if (this.mode === 'enter') {
        this.actionButton.textContent = 'Dive';
      } else {
        this.actionButton.textContent = 'Action';
      }
    }
    if (this.menuButton) this.menuButton.setAttribute('aria-expanded', String(this.menuOpen));
    if (this.menuPanel) this.menuPanel.hidden = !this.menuOpen;
    if (this.actionLabelsButton) this.actionLabelsButton.textContent = this.labelsVisible ? 'Labels Off' : 'Labels On';
    const selectedNode = this.nodes.get(this.selectedNodeId);
    if (this.renameInput) {
      this.renameInput.disabled = !selectedNode;
      this.renameInput.value = selectedNode ? selectedNode.label : '';
      this.renameInput.placeholder = selectedNode ? 'Rename selected node' : 'Select node to rename';
    }
    if (this.renameApplyButton) this.renameApplyButton.disabled = !selectedNode;
    this.canvas.setAttribute('data-action-mode', this.mode);
    this.canvas.setAttribute('data-labels-visible', String(this.labelsVisible));
    this.canvas.setAttribute('data-connect-pending', this.pendingConnectSourceNodeId);
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
      console.log(`[Three #three] selected node: ${nodeId}`);
    }
    this.refreshVisualState();
    this.syncCanvasState();
  }

  private applyLayerView(layerId: LayerID) {
    this.activeLayerId = layerId;
    const visibleLayerIDs = new Set<LayerID>();
    visibleLayerIDs.add(layerId);
    const activeLayer = this.layers.get(layerId);
    if (activeLayer?.parentNodeId) {
      const parentNode = this.nodes.get(activeLayer.parentNodeId);
      if (parentNode) visibleLayerIDs.add(parentNode.layerId);
    }

    for (const node of this.nodes.values()) {
      const isVisible = visibleLayerIDs.has(node.layerId);
      const isActive = node.layerId === layerId;
      node.mesh.visible = isVisible;
      node.labelMesh.visible = isVisible && this.labelsVisible;
      const material = node.mesh.material as THREE.MeshStandardMaterial;
      material.transparent = !isActive;
      material.opacity = isActive ? 1 : 0.24;
    }
    for (const edge of this.edges.values()) {
      const isVisible = visibleLayerIDs.has(edge.layerId);
      const isActive = edge.layerId === layerId;
      edge.line.visible = isVisible;
      const material = edge.line.material as THREE.LineBasicMaterial;
      material.transparent = !isActive;
      material.opacity = isActive ? 1 : 0.2;
      this.updateEdgeLine(edge);
    }
    for (const link of this.nestedLinks) {
      this.updateNestedLinkGeometry(link);
      link.line.visible = link.childLayerId === layerId;
    }
    this.fitCameraToLayer(layerId);
    this.refreshVisualState();
    this.syncCanvasState();
  }

  private fitCameraToLayer(layerId: LayerID) {
    const defaultView: CameraView = this.camera.aspect < 0.7 ? 'front' : 'iso';
    this.setCameraViewForLayer(layerId, defaultView);
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
    return { ok: true, center, maxDim: Math.max(4, size.x, size.y) };
  }

  private setCameraViewForLayer(layerId: LayerID, view: CameraView): boolean {
    const bounds = this.getLayerBounds(layerId);
    if (!bounds.ok) return false;
    const fov = THREE.MathUtils.degToRad(this.camera.fov);
    const aspectScale = this.camera.aspect < 1 ? 1 / this.camera.aspect : 1;
    const dist = ((bounds.maxDim * aspectScale) / (2 * Math.tan(fov / 2))) * 1.2 + 14;
    const c = bounds.center;

    switch (view) {
      case 'top':
        this.camera.position.set(c.x, c.y + dist, c.z + 0.01);
        break;
      case 'side':
        this.camera.position.set(c.x + dist, c.y, c.z + 0.01);
        break;
      case 'iso':
        this.camera.position.set(c.x + dist*0.75, c.y + dist*0.65, c.z + dist*0.75);
        break;
      case 'front':
      default:
        this.camera.position.set(c.x, c.y, c.z + dist);
        break;
    }

    this.camera.lookAt(c);
    this.camera.updateProjectionMatrix();
    return true;
  }

  private refreshVisualState() {
    const activeLayer = this.layers.get(this.activeLayerId);
    if (!activeLayer) return;

    const inputNodeIDs = new Set(this.getInputNodeIDs(this.selectedNodeId));
    const outputNodeIDs = new Set(this.getOutputNodeIDs(this.selectedNodeId));

    for (const nodeId of activeLayer.nodeIds) {
      const node = this.nodes.get(nodeId);
      if (!node) continue;
      const material = node.mesh.material as THREE.MeshStandardMaterial;
      if (node.id === this.selectedNodeId) {
        material.color.setHex(NODE_SELECTED_COLOR);
      } else if (inputNodeIDs.has(node.id)) {
        material.color.setHex(NODE_INPUT_COLOR);
      } else if (outputNodeIDs.has(node.id)) {
        material.color.setHex(NODE_OUTPUT_COLOR);
      } else if (node.layerId !== 'root') {
        material.color.setHex(NODE_NESTED_LAYER_COLOR);
      } else if (node.nestedLayerId) {
        material.color.setHex(NODE_NESTED_COLOR);
      } else {
        material.color.setHex(NODE_BASE_COLOR);
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
    if (!node || !node.nestedLayerId) return [];
    const layer = this.layers.get(node.nestedLayerId);
    return layer ? [...layer.nodeIds].sort() : [];
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

  private enterNested(nodeId?: NodeID): boolean {
    const targetNode = this.nodes.get(nodeId || this.selectedNodeId);
    if (!targetNode || !targetNode.nestedLayerId) return false;
    if (!this.layers.has(targetNode.nestedLayerId)) return false;
    this.history.push({ layerId: this.activeLayerId, selectedNodeId: this.selectedNodeId });
    this.selectedNodeId = '';
    this.applyLayerView(targetNode.nestedLayerId);
    console.log(`[Three #three] enter nested layer: ${targetNode.nestedLayerId}`);
    return true;
  }

  private goBack(): boolean {
    const prev = this.history.pop();
    if (!prev) return false;
    this.selectedNodeId = prev.selectedNodeId;
    this.applyLayerView(prev.layerId);
    console.log(`[Three #three] back to layer: ${prev.layerId}`);
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
    node.mesh.position.set(this.rankToX(node.rank), this.rowToY(node.row), 0);
    node.labelMesh.position.set(this.rankToX(node.rank) + 2.25, this.rowToY(node.row), 0);
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
    const plane = new THREE.Plane(new THREE.Vector3(0, 0, 1), 0);
    const world = new THREE.Vector3();
    if (!this.raycaster.ray.intersectPlane(plane, world)) return '';

    const approxRank = Math.max(0, Math.round((world.x + this.rankXSpacing) / this.rankXSpacing));
    const approxRow = Math.max(0, Math.round((layer.baseY - world.y) / this.rowYSpacing));
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

    if (node.nestedLayerId) {
      const nestedLayer = this.layers.get(node.nestedLayerId);
      if (nestedLayer) {
        for (const nestedNodeID of [...nestedLayer.nodeIds]) this.removeNode(nestedNodeID);
        for (const nestedEdgeID of [...nestedLayer.edgeIds]) this.removeEdge(nestedEdgeID);
        this.layers.delete(node.nestedLayerId);
      }
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
      mode: this.mode,
      labelsVisible: this.labelsVisible,
      pendingConnectSourceNodeId: this.pendingConnectSourceNodeId,
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
    return this.setCameraViewForLayer(this.activeLayerId, view);
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

  private attachDebugBridge() {
    (window as Window & { dagHitTestDebug?: Record<string, unknown> }).dagHitTestDebug = {
      getState: () => this.getState(),
      getProjectedPoint: (nodeId: NodeID) => this.getProjectedPoint(nodeId),
      getNodeWorldPosition: (nodeId: NodeID) => this.getNodeWorldPosition(nodeId),
      getNodeLabel: (nodeId: NodeID) => this.nodes.get(nodeId)?.label ?? '',
      getNodeTransform: (nodeId: NodeID) => this.getNodeTransform(nodeId),
      getLayerTransform: (layerId: LayerID) => this.getLayerTransform(layerId),
      getCameraTransform: () => this.getCameraTransform(),
      setCameraView: (view: CameraView) => this.setCameraView(view),
      logLayoutSnapshot: (layerId: LayerID, nodeIDs: NodeID[]) => this.logLayoutSnapshot(layerId, nodeIDs),
      clickProjected: (nodeId: NodeID) => this.clickProjected(nodeId),
      createNodeAtClient: (layerId: LayerID, clientX: number, clientY: number) => this.createNodeAtClient(layerId, clientX, clientY),
      enterNested: (nodeId?: NodeID) => this.enterNested(nodeId),
      goBack: () => this.goBack(),
      setMode: (mode: ActionMode) => {
        if (!MODE_SEQUENCE.includes(mode)) return false;
        this.mode = mode;
        this.pendingConnectSourceNodeId = '';
        this.syncControlState();
        return true;
      },
      toggleLabels: () => this.toggleLabels(),
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
    this.renderer.dispose();
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']") as HTMLCanvasElement | null;
  if (!canvas) throw new Error('three canvas not found');
  return new ThreeControl(container, canvas);
}
