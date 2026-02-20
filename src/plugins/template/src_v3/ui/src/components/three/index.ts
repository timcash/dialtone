import * as THREE from 'three';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { VisualizationControl } from '../../../../../../../plugins/ui/types';

type ThumbMode = 'graph' | 'view';
type ThreeNode = { id: string; mesh: THREE.Mesh };

class ThreeControl implements VisualizationControl {
  private readonly scene = new THREE.Scene();
  private readonly camera = new THREE.PerspectiveCamera(50, 1, 0.1, 400);
  private readonly renderer: THREE.WebGLRenderer;
  private readonly nodes: ThreeNode[] = [];
  private selectedNodeId = '';
  private frameID = 0;
  private visible = false;
  private nodeCounter = 1;
  private mode: ThumbMode = 'graph';
  private history: string[] = [];
  private labelsVisible = true;

  private buttons: HTMLButtonElement[] = [];
  private modeButton: HTMLButtonElement | null = null;
  private inputEl: HTMLInputElement | null = null;
  private submitEl: HTMLButtonElement | null = null;
  private historyTitle: HTMLElement | null = null;
  private historyEls: HTMLElement[] = [];
  private chatlogHost: HTMLElement | null = null;
  private chatlogTerm: Terminal | null = null;
  private chatlogLines: string[] = [];

  constructor(private readonly container: HTMLElement, private readonly canvas: HTMLCanvasElement) {
    this.renderer = new THREE.WebGLRenderer({ canvas: this.canvas, antialias: true });
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.setClearColor(0x05070a, 1);
    this.scene.add(new THREE.AmbientLight(0xffffff, 0.5));
    const key = new THREE.DirectionalLight(0xffffff, 0.9);
    key.position.set(12, 16, 10);
    this.scene.add(key);
    this.camera.position.set(0, 12, 22);
    this.camera.lookAt(0, 0, 0);

    this.initUI();
    this.seedNodes();
    this.resize();
    this.attachEvents();
    this.animate();
    this.canvas.setAttribute('data-ready', 'true');
  }

  private initUI() {
    this.buttons = [
      this.container.querySelector("button[aria-label='Three Back']"),
      this.container.querySelector("button[aria-label='Three Add']"),
      this.container.querySelector("button[aria-label='Three Link']"),
      this.container.querySelector("button[aria-label='Three Clear']"),
      this.container.querySelector("button[aria-label='Three Open']"),
      this.container.querySelector("button[aria-label='Three Rename']"),
      this.container.querySelector("button[aria-label='Three Focus']"),
      this.container.querySelector("button[aria-label='Three Labels']"),
    ].filter((el): el is HTMLButtonElement => !!el);
    this.modeButton = this.container.querySelector("button[aria-label='Three Mode']");
    this.inputEl = this.container.querySelector("input[aria-label='Three Label Input']");
    this.submitEl = this.container.querySelector("button[aria-label='Three Submit']");
    this.historyTitle = this.container.querySelector('.three-history > h3');
    this.historyEls = [
      this.container.querySelector("[aria-label='Three Node History Item 1']"),
      this.container.querySelector("[aria-label='Three Node History Item 2']"),
      this.container.querySelector("[aria-label='Three Node History Item 3']"),
      this.container.querySelector("[aria-label='Three Node History Item 4']"),
      this.container.querySelector("[aria-label='Three Node History Item 5']"),
    ].filter((el): el is HTMLElement => !!el);
    this.chatlogHost = this.container.querySelector('.three-chatlog-xterm');
    this.initChatlog();

    this.buttons[0]?.addEventListener('click', () => this.selectFromHistory(1));
    this.buttons[1]?.addEventListener('click', () => this.addNodeFromSelection());
    this.buttons[2]?.addEventListener('click', () => this.selectNext());
    this.buttons[3]?.addEventListener('click', () => this.clearSelection());
    this.buttons[4]?.addEventListener('click', () => this.selectFirst());
    this.buttons[5]?.addEventListener('click', () => this.renameSelectedFromInput());
    this.buttons[6]?.addEventListener('click', () => this.focusSelection());
    this.buttons[7]?.addEventListener('click', () => this.toggleLabels());
    this.modeButton?.addEventListener('click', () => this.toggleMode());
    this.submitEl?.addEventListener('click', () => this.renameSelectedFromInput());
    this.inputEl?.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') return;
      event.preventDefault();
      this.renameSelectedFromInput();
    });
    this.syncButtons();
  }

  private initChatlog() {
    if (!this.chatlogHost) return;
    this.chatlogTerm?.dispose();
    this.chatlogHost.innerHTML = '';
    this.chatlogTerm = new Terminal({
      allowTransparency: true,
      convertEol: true,
      disableStdin: true,
      cursorBlink: false,
      rows: 7,
      cols: 90,
      scrollback: 0,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace',
      fontSize: 12,
      lineHeight: 1.25,
      theme: { background: 'rgba(0,0,0,0)', foreground: '#a7adb7', cursor: '#a7adb7' },
    });
    this.chatlogTerm.open(this.chatlogHost);
  }

  private appendLog(line: string) {
    const clean = line.replace(/\s+/g, ' ').trim();
    if (!clean) return;
    this.chatlogLines.push(clean);
    if (this.chatlogLines.length > 7) this.chatlogLines = this.chatlogLines.slice(-7);
    this.chatlogTerm?.write(`\x1b[2J\x1b[H${this.chatlogLines.join('\r\n')}`);
  }

  private seedNodes() {
    this.addNodeAt('N1', new THREE.Vector3(-4, 0, 0));
    this.addNodeAt('N2', new THREE.Vector3(0, 0, 0));
    this.addNodeAt('N3', new THREE.Vector3(4, 0, 0));
    this.selectNode('N2');
  }

  private addNodeAt(id: string, position: THREE.Vector3) {
    const geometry = new THREE.BoxGeometry(1.5, 1.5, 1.5);
    const material = new THREE.MeshStandardMaterial({ color: 0x4f8df8, roughness: 0.45, metalness: 0.2 });
    const mesh = new THREE.Mesh(geometry, material);
    mesh.position.copy(position);
    mesh.userData = { nodeId: id };
    this.scene.add(mesh);
    this.nodes.push({ id, mesh });
  }

  private selectNode(nodeId: string) {
    this.selectedNodeId = nodeId;
    this.canvas.setAttribute('data-selected-node', nodeId);
    if (nodeId) {
      this.history = [nodeId, ...this.history.filter((id) => id !== nodeId)].slice(0, 5);
      this.appendLog(`USER> Selected ${nodeId}`);
    }
    this.paintSelection();
    this.renderHistory();
    this.syncButtons();
  }

  private paintSelection() {
    for (const node of this.nodes) {
      const mat = node.mesh.material as THREE.MeshStandardMaterial;
      const selected = node.id === this.selectedNodeId;
      mat.color.setHex(selected ? 0xf3f8ff : 0x4f8df8);
      mat.emissive.setHex(selected ? 0x355ca8 : 0x000000);
      mat.emissiveIntensity = selected ? 0.75 : 0.15;
    }
  }

  private renderHistory() {
    if (this.historyTitle) this.historyTitle.textContent = `Node History ${this.history.length}/5`;
    for (let i = 0; i < this.historyEls.length; i += 1) {
      this.historyEls[i].textContent = this.history[i] ?? 'none';
    }
  }

  private addNodeFromSelection() {
    const anchor = this.nodes.find((n) => n.id === this.selectedNodeId) ?? this.nodes[this.nodes.length - 1];
    const id = `N${this.nodeCounter + 3}`;
    this.nodeCounter += 1;
    const next = anchor ? anchor.mesh.position.clone().add(new THREE.Vector3(3.2, 0, 0)) : new THREE.Vector3(0, 0, 0);
    this.addNodeAt(id, next);
    this.selectNode(id);
    this.appendLog(`USER> Click Three Add`);
  }

  private selectFromHistory(index: number) {
    const id = this.history[index];
    if (id) this.selectNode(id);
  }

  private selectNext() {
    if (this.nodes.length === 0) return;
    const currentIndex = this.nodes.findIndex((n) => n.id === this.selectedNodeId);
    const next = this.nodes[(currentIndex + 1 + this.nodes.length) % this.nodes.length];
    this.selectNode(next.id);
    this.appendLog(`USER> Click Three Link`);
  }

  private clearSelection() {
    this.selectedNodeId = '';
    this.canvas.setAttribute('data-selected-node', '');
    this.paintSelection();
    this.appendLog(`USER> Click Three Clear`);
  }

  private selectFirst() {
    const first = this.nodes[0];
    if (first) this.selectNode(first.id);
    this.appendLog(`USER> Click Three Open`);
  }

  private renameSelectedFromInput() {
    const id = this.selectedNodeId;
    const next = (this.inputEl?.value ?? '').trim();
    if (!id || !next) return;
    this.history = this.history.map((item) => (item === id ? next : item));
    this.selectedNodeId = next;
    this.canvas.setAttribute('data-selected-node', next);
    this.inputEl!.value = '';
    this.renderHistory();
    this.appendLog(`USER> Rename ${id} -> ${next}`);
  }

  private focusSelection() {
    const node = this.nodes.find((n) => n.id === this.selectedNodeId);
    if (!node) return;
    this.camera.position.set(node.mesh.position.x, 10, node.mesh.position.z + 16);
    this.camera.lookAt(node.mesh.position.x, 0, node.mesh.position.z);
    this.appendLog(`USER> Click Three Focus`);
  }

  private toggleLabels() {
    this.labelsVisible = !this.labelsVisible;
    this.appendLog(`USER> Labels ${this.labelsVisible ? 'On' : 'Off'}`);
    this.syncButtons();
  }

  private toggleMode() {
    this.mode = this.mode === 'graph' ? 'view' : 'graph';
    this.appendLog(`USER> Mode ${this.mode}`);
    this.syncButtons();
  }

  private syncButtons() {
    if (this.modeButton) this.modeButton.textContent = this.mode === 'graph' ? '9:Mode: Build' : '9:Mode: View';
    const labels = this.mode === 'graph' ? ['1:Back', '2:Add', '3:Link', '4:Clear', '5:Open', '6:Rename', '7:Focus', '8:Labels On'] : ['1:Back', '2:Add', '3:Link', '4:Clear', '5:Open', '6:Rename', '7:Focus', '8:Labels Off'];
    for (let i = 0; i < this.buttons.length; i += 1) this.buttons[i].textContent = labels[i];
  }

  private attachEvents() {
    window.addEventListener('resize', this.resize);
    this.canvas.addEventListener('click', this.onCanvasClick);
  }

  private onCanvasClick = (event: MouseEvent) => {
    const rect = this.canvas.getBoundingClientRect();
    const x = ((event.clientX - rect.left) / rect.width) * 2 - 1;
    const y = -((event.clientY - rect.top) / rect.height) * 2 + 1;
    const raycaster = new THREE.Raycaster();
    raycaster.setFromCamera(new THREE.Vector2(x, y), this.camera);
    const hits = raycaster.intersectObjects(this.nodes.map((n) => n.mesh), false);
    const hit = hits[0]?.object?.userData?.nodeId;
    if (typeof hit === 'string') this.selectNode(hit);
  };

  private animate = () => {
    this.frameID = window.requestAnimationFrame(this.animate);
    if (!this.visible) return;
    this.renderer.render(this.scene, this.camera);
  };

  private resize = () => {
    const width = this.canvas.clientWidth || this.container.clientWidth || window.innerWidth;
    const height = this.canvas.clientHeight || this.container.clientHeight || window.innerHeight;
    if (width <= 0 || height <= 0) return;
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    this.renderer.setSize(width, height, false);
  };

  setVisible(visible: boolean) {
    this.visible = visible;
    this.canvas.style.visibility = visible ? 'visible' : 'hidden';
    if (visible) this.resize();
  }

  dispose() {
    window.cancelAnimationFrame(this.frameID);
    window.removeEventListener('resize', this.resize);
    this.canvas.removeEventListener('click', this.onCanvasClick);
    this.chatlogTerm?.dispose();
    this.renderer.dispose();
  }
}

export function mountThree(container: HTMLElement): VisualizationControl {
  const canvas = container.querySelector("canvas[aria-label='Three Canvas']");
  if (!canvas || !(canvas instanceof HTMLCanvasElement)) {
    throw new Error('Three Canvas not found');
  }
  return new ThreeControl(container, canvas);
}
