type TestResult = { ok: boolean; msg: string };
type StoryStore = Record<string, unknown>;

type DagDebugAPI = {
  getState: () => {
    activeLayerId: string;
    selectedNodeId: string;
    visibleNodeIDs: string[];
    inputNodeIDs: string[];
    outputNodeIDs: string[];
    historyDepth: number;
    recentSelectedNodeIDs: string[];
    lastCreatedNodeId: string;
    selectedNodeLabel: string;
    camera: { x: number; y: number; z: number };
  };
  getProjectedPoint: (nodeId: string) => { ok: boolean; x: number; y: number };
  getNodeColorHex: (nodeId: string) => { ok: boolean; colorHex: number };
  getNodeTransform: (nodeId: string) => {
    ok: boolean;
    rank: number;
    position: { x: number; y: number; z: number };
  };
  getLayerTransform: (layerId: string) => {
    ok: boolean;
    position: { x: number; y: number; z: number };
  };
  getNodeLabel: (nodeId: string) => string;
  setCameraView: (view: string) => boolean;
};

function q(label: string): HTMLElement | null {
  return document.querySelector(`[aria-label='${label}']`);
}

function isPressed(label: string): boolean {
  const el = q(label);
  return !!el && el.getAttribute('aria-pressed') === 'true';
}

function testClickDelayMS(): number {
  try {
    const v = window.sessionStorage.getItem('dag_test_click_delay_ms') || '0';
    const n = Number(v);
    return Number.isFinite(n) ? Math.max(0, n) : 0;
  } catch {
    return 0;
  }
}

function maybePaceClick(): void {
  const delay = testClickDelayMS();
  if (delay <= 0) return;
  const start = performance.now();
  while (performance.now() - start < delay) {
    // busy-wait pacing for attach mode
  }
}

function click(label: string): boolean {
  const el = q(label) as HTMLButtonElement | null;
  if (!el) return false;
  el.click();
  maybePaceClick();
  return true;
}

function clickNode(api: DagDebugAPI, nodeId: string): boolean {
  const p = api.getProjectedPoint(nodeId);
  if (!p?.ok) return false;
  const canvas = q('Three Canvas');
  if (!canvas) return false;
  canvas.dispatchEvent(
    new MouseEvent('click', { bubbles: true, cancelable: true, clientX: p.x, clientY: p.y, view: window })
  );
  maybePaceClick();
  return api.getState().selectedNodeId === nodeId;
}

function historyValue(label: string): string {
  const el = q(label);
  return el ? String(el.textContent || '').trim() : '';
}

function nodeColorHex(api: DagDebugAPI, nodeId: string): number {
  const info = api.getNodeColorHex(nodeId);
  return info && info.ok ? info.colorHex : -1;
}

function setName(text: string): boolean {
  const input = q('DAG Label Input') as HTMLInputElement | null;
  if (!input) return false;
  input.value = text;
  input.dispatchEvent(new Event('input', { bubbles: true }));
  return click('DAG Rename');
}

function story(): StoryStore {
  const win = window as any;
  win.__dagStory = win.__dagStory || {};
  return win.__dagStory as StoryStore;
}

function requireAPI(): DagDebugAPI | null {
  const api = (window as any).dagHitTestDebug;
  if (!api || typeof api.getState !== 'function') return null;
  return api as DagDebugAPI;
}

function runStoryStep1(api: DagDebugAPI): TestResult {
  const initial = api.getState();
  if (!initial || initial.activeLayerId !== 'root') return { ok: false, msg: 'initial root layer missing' };
  if (initial.visibleNodeIDs.length !== 0) return { ok: false, msg: 'expected empty dag at start' };
  if (!click('DAG Add')) return { ok: false, msg: 'add action failed' };

  const afterAdd = api.getState();
  if (afterAdd.visibleNodeIDs.length !== 1) return { ok: false, msg: 'first node not created' };
  const processorID = afterAdd.lastCreatedNodeId;
  if (!processorID) return { ok: false, msg: 'missing processor node id' };
  if (afterAdd.selectedNodeId !== processorID) return { ok: false, msg: 'new node not selected' };

  const moved = (a: { x: number; y: number; z: number }, b: { x: number; y: number; z: number }) =>
    Math.abs(a.x - b.x) >= 0.5 || Math.abs(a.y - b.y) >= 0.5 || Math.abs(a.z - b.z) >= 0.5;
  const camISO = afterAdd.camera;
  if (!click('DAG Camera Z')) return { ok: false, msg: 'camera z button failed' };
  const camTop = api.getState().camera;
  if (!moved(camISO, camTop)) return { ok: false, msg: 'camera did not move on z view' };
  if (!isPressed('DAG Camera Z') || isPressed('DAG Camera ISO') || isPressed('DAG Camera Side')) {
    return { ok: false, msg: 'camera z button highlight state invalid' };
  }
  if (!click('DAG Camera Side')) return { ok: false, msg: 'camera side button failed' };
  const camSide = api.getState().camera;
  if (!moved(camTop, camSide)) return { ok: false, msg: 'camera did not move on side view' };
  if (!isPressed('DAG Camera Side') || isPressed('DAG Camera Z') || isPressed('DAG Camera ISO')) {
    return { ok: false, msg: 'camera side button highlight state invalid' };
  }
  if (!click('DAG Camera ISO')) return { ok: false, msg: 'camera iso button failed' };
  const camISO2 = api.getState().camera;
  if (!moved(camSide, camISO2)) return { ok: false, msg: 'camera did not move on iso view' };
  if (!isPressed('DAG Camera ISO') || isPressed('DAG Camera Z') || isPressed('DAG Camera Side')) {
    return { ok: false, msg: 'camera iso button highlight state invalid' };
  }
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot reselect processor after camera view changes' };
  if (!isPressed('DAG Camera ISO')) return { ok: false, msg: 'camera style did not persist after node reselection' };

  const s = story();
  s.processorID = processorID;
  s.rootCameraBeforeOpen = afterAdd.camera;
  return { ok: true, msg: 'ok' };
}

function runStoryStep2(api: DagDebugAPI): TestResult {
  const s = story();
  const processorID = String(s.processorID || '');
  if (!processorID) return { ok: false, msg: 'missing processor id from step1' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor' };

  if (!click('DAG Add')) return { ok: false, msg: 'add output failed' };
  let st = api.getState();
  const outputID = st.lastCreatedNodeId;
  if (!outputID || outputID === processorID) return { ok: false, msg: 'missing output node id' };
  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before connect processor->output failed' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor for output link source' };
  if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'history item1 did not show processor' };
  if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be none after first selection' };
  if (nodeColorHex(api, processorID) !== 0x7dd3fc) return { ok: false, msg: 'most recent node color should be light blue' };
  if (!clickNode(api, outputID)) return { ok: false, msg: 'cannot select output node for output link target' };
  if (historyValue('DAG Node History Item 1') !== outputID) return { ok: false, msg: 'history item1 did not show output node after second selection' };
  if (historyValue('DAG Node History Item 2') !== processorID) return { ok: false, msg: 'history item2 did not show processor after second selection' };
  if (nodeColorHex(api, outputID) !== 0x7dd3fc) return { ok: false, msg: 'most recent node color should stay light blue after second selection' };
  if (nodeColorHex(api, processorID) !== 0x2b78ff) return { ok: false, msg: 'second most recent node color should be blue' };
  if (!click('DAG Connect')) return { ok: false, msg: 'connect processor->output failed' };

  const canvas = q('Three Canvas');
  if (!canvas) return { ok: false, msg: 'missing three canvas' };
  canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: 8, clientY: 8, view: window }));
  if (api.getState().selectedNodeId !== '') return { ok: false, msg: 'failed to clear selection' };
  if (!click('DAG Add')) return { ok: false, msg: 'add input failed' };
  st = api.getState();
  const inputID = st.lastCreatedNodeId;
  if (!inputID || inputID === outputID || inputID === processorID) return { ok: false, msg: 'missing input node id' };

  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before connect failed' };
  if (!clickNode(api, inputID)) return { ok: false, msg: 'cannot select input node' };
  if (historyValue('DAG Node History Item 1') !== inputID) return { ok: false, msg: 'history item1 did not show input node after first selection' };
  if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be none before second selection' };
  if (nodeColorHex(api, inputID) !== 0x7dd3fc) return { ok: false, msg: 'new most recent node should be light blue' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor for connect target' };
  if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'history item1 did not show processor after second selection' };
  if (historyValue('DAG Node History Item 2') !== inputID) return { ok: false, msg: 'history item2 did not show input node after second selection' };
  if (nodeColorHex(api, processorID) !== 0x7dd3fc) return { ok: false, msg: 'processor should be light blue when most recent' };
  if (nodeColorHex(api, inputID) !== 0x2b78ff) return { ok: false, msg: 'input node should be blue when second most recent' };
  if (nodeColorHex(api, outputID) !== 0x5b6873) return { ok: false, msg: 'older nodes should be gray' };
  if (!click('DAG Connect')) return { ok: false, msg: 'connect apply failed' };

  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot reselect processor' };
  st = api.getState();
  if (!st.inputNodeIDs.includes(inputID)) return { ok: false, msg: 'processor missing input edge' };
  if (!st.outputNodeIDs.includes(outputID)) return { ok: false, msg: 'processor missing output edge' };
  const inputTx = api.getNodeTransform(inputID);
  const processorTx = api.getNodeTransform(processorID);
  if (!inputTx.ok || !processorTx.ok) return { ok: false, msg: 'missing node transforms for rank checks' };
  if (processorTx.rank < inputTx.rank+1) return { ok: false, msg: 'rank rule violated: input node did not move above highest input rank' };

  s.inputID = inputID;
  s.outputID = outputID;
  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before screenshot setup failed' };
  if (!clickNode(api, inputID)) return { ok: false, msg: 'cannot select input for screenshot first pick' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor for screenshot second pick' };
  if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'screenshot history item1 mismatch' };
  if (historyValue('DAG Node History Item 2') !== inputID) return { ok: false, msg: 'screenshot history item2 mismatch' };
  if (!click('DAG Back')) return { ok: false, msg: 'back action for history pop failed' };
  const afterBack = api.getState();
  if (afterBack.selectedNodeId !== inputID) return { ok: false, msg: 'back should select previous node from history' };
  if (historyValue('DAG Node History Item 1') !== inputID) return { ok: false, msg: 'history item1 should pop to previous node after back' };
  if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be cleared after single back pop' };
  if (!afterBack.camera || afterBack.camera.y < 20) return { ok: false, msg: 'camera too low after back' };
  if (!api.setCameraView('iso')) return { ok: false, msg: 'failed to reset camera after back assertion' };
  return { ok: true, msg: 'ok' };
}

function runStoryStep3(api: DagDebugAPI): TestResult {
  const s = story();
  const processorID = String(s.processorID || '');
  if (!processorID) return { ok: false, msg: 'missing processor id' };
  const rootLayerTx = api.getLayerTransform('root');
  if (!rootLayerTx?.ok) return { ok: false, msg: 'root layer transform missing before open' };
  const processorTx = api.getNodeTransform(processorID);
  if (!processorTx?.ok) return { ok: false, msg: 'processor transform missing before open' };

  const rootBeforeOpen = api.getState().camera;
  if (!clickNode(api, processorID)) return { ok: false, msg: 'select processor failed' };
  if (!click('DAG Nest')) return { ok: false, msg: 'nest action failed' };
  let st = api.getState();
  if (!st.activeLayerId || st.activeLayerId === 'root') return { ok: false, msg: 'did not open nested layer' };
  if (st.historyDepth < 1) return { ok: false, msg: 'history depth not increased on open' };
  const nestedLayerID = st.activeLayerId;
  const nestedLayerTx = api.getLayerTransform(nestedLayerID);
  if (!nestedLayerTx?.ok) return { ok: false, msg: 'nested layer transform missing after open' };
  if (nestedLayerTx.position.y <= rootLayerTx.position.y + 1) return { ok: false, msg: 'nested layer should be above root on y axis' };
  if (Math.abs(nestedLayerTx.position.x - processorTx.position.x) > 0.2) return { ok: false, msg: 'nested layer x should align with parent node x' };
  if (Math.abs(nestedLayerTx.position.z - processorTx.position.z) > 0.2) return { ok: false, msg: 'nested layer z should align with parent node z' };
  if (!st.camera || st.camera.y <= rootBeforeOpen.y + 1) return { ok: false, msg: 'camera did not elevate on nested open' };

  s.nestedLayerID = nestedLayerID;
  s.rootCameraBeforeOpen = rootBeforeOpen;
  s.nestedCameraAfterOpen = st.camera;

  if (!click('DAG Add')) return { ok: false, msg: 'nested add node A failed' };
  st = api.getState();
  const nestedA = st.lastCreatedNodeId;
  if (!nestedA) return { ok: false, msg: 'nested node A id missing' };
  if (!click('DAG Add')) return { ok: false, msg: 'nested add node B failed' };
  st = api.getState();
  const nestedB = st.lastCreatedNodeId;
  if (!nestedB || nestedB === nestedA) return { ok: false, msg: 'nested node B id missing' };

  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before nested link failed' };
  if (!clickNode(api, nestedA)) return { ok: false, msg: 'select nestedA for link source failed' };
  if (!clickNode(api, nestedB)) return { ok: false, msg: 'select nestedB for link target failed' };
  if (!click('DAG Connect')) return { ok: false, msg: 'connect nestedA->nestedB failed' };
  if (!clickNode(api, nestedA)) return { ok: false, msg: 'reselect nestedA failed' };
  st = api.getState();
  if (!st.outputNodeIDs.includes(nestedB)) return { ok: false, msg: 'nestedA missing output to nestedB' };

  s.nestedA = nestedA;
  s.nestedB = nestedB;
  return { ok: true, msg: 'ok' };
}

function runStoryStep4(api: DagDebugAPI): TestResult {
  const s = story();
  const nestedA = String(s.nestedA || '');
  const processorID = String(s.processorID || '');
  if (!nestedA || !processorID) return { ok: false, msg: 'missing story ids' };
  if (api.getState().activeLayerId === 'root') return { ok: false, msg: 'expected to start in nested layer' };

  if (!clickNode(api, nestedA)) return { ok: false, msg: 'select nestedA failed' };
  if (!setName('Nested Input')) return { ok: false, msg: 'rename nestedA action failed' };
  if (api.getNodeLabel(nestedA) !== 'Nested Input') return { ok: false, msg: 'nested label did not update' };

  const nestedCamera = api.getState().camera;
  if (!click('DAG Back')) return { ok: false, msg: 'back action failed' };
  const rootState = api.getState();
  if (rootState.activeLayerId !== 'root') return { ok: false, msg: 'did not return to root layer' };
  if (rootState.historyDepth !== 0) return { ok: false, msg: 'history not cleared after close' };
  if (rootState.selectedNodeId !== processorID) return { ok: false, msg: 'close should focus parent node selection' };
  if (rootState.recentSelectedNodeIDs.includes(nestedA)) return { ok: false, msg: 'closed layer nodes should be removed from selection history' };
  const camMoved =
    Math.abs(rootState.camera.x - nestedCamera.x) >= 1 ||
    Math.abs(rootState.camera.y - nestedCamera.y) >= 1 ||
    Math.abs(rootState.camera.z - nestedCamera.z) >= 1;
  if (!camMoved) return { ok: false, msg: 'camera did not move on close' };

  if (!clickNode(api, processorID)) return { ok: false, msg: 'select processor after close failed' };
  if (!setName('Processor')) return { ok: false, msg: 'rename processor action failed' };
  if (api.getNodeLabel(processorID) !== 'Processor') return { ok: false, msg: 'processor label did not update' };
  if (api.getState().selectedNodeLabel !== 'Processor') return { ok: false, msg: 'selected node label state mismatch' };
  return { ok: true, msg: 'ok' };
}

function runStoryStep5(api: DagDebugAPI): TestResult {
  const s = story();
  const processorID = String(s.processorID || '');
  const nestedB = String(s.nestedB || '');
  if (!processorID || !nestedB) return { ok: false, msg: 'missing story ids for deep nesting' };

  if (!clickNode(api, processorID)) return { ok: false, msg: 'select processor failed' };
  if (!click('DAG Nest')) return { ok: false, msg: 'enter processor nested layer failed' };
  let st = api.getState();
  if (st.historyDepth < 1) return { ok: false, msg: 'history depth missing after first open' };
  const level1LayerTx = api.getLayerTransform(st.activeLayerId);
  if (!level1LayerTx?.ok) return { ok: false, msg: 'level1 layer transform missing' };

  if (!clickNode(api, nestedB)) return { ok: false, msg: 'select nestedB failed' };
  if (!click('DAG Nest')) return { ok: false, msg: 'create + enter second-level nested layer failed' };
  st = api.getState();
  if (st.historyDepth < 2) return { ok: false, msg: 'history depth missing after second open' };
  const level2LayerID = st.activeLayerId;
  if (!level2LayerID || level2LayerID === 'root') return { ok: false, msg: 'missing level2 active layer id' };
  const level2LayerTx = api.getLayerTransform(level2LayerID);
  if (!level2LayerTx?.ok) return { ok: false, msg: 'level2 layer transform missing' };
  if (level2LayerTx.position.y <= level1LayerTx.position.y + 1) return { ok: false, msg: 'level2 layer should be above level1 on y axis' };
  const nestedCam = s.nestedCameraAfterOpen as { y: number } | undefined;
  if (!st.camera || st.camera.y <= ((nestedCam?.y ?? -Infinity) + 1)) return { ok: false, msg: 'camera did not elevate for second-level nested open' };

  if (!click('DAG Add')) return { ok: false, msg: 'add level2 node A failed' };
  st = api.getState();
  const level2A = st.lastCreatedNodeId;
  if (!level2A) return { ok: false, msg: 'missing level2A id' };
  if (!click('DAG Add')) return { ok: false, msg: 'add level2 node B failed' };
  st = api.getState();
  const level2B = st.lastCreatedNodeId;
  if (!level2B || level2B === level2A) return { ok: false, msg: 'missing level2B id' };
  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before level2 connect failed' };
  if (!clickNode(api, level2A)) return { ok: false, msg: 'select level2A failed' };
  if (!clickNode(api, level2B)) return { ok: false, msg: 'select level2B failed' };
  if (!click('DAG Connect')) return { ok: false, msg: 'apply level2 connect failed' };
  st = api.getState();
  if (!st.inputNodeIDs.includes(level2A)) return { ok: false, msg: 'level2 edge missing' };

  s.level2LayerID = level2LayerID;
  s.level2A = level2A;
  s.level2B = level2B;
  s.level2Camera = st.camera;
  return { ok: true, msg: 'ok' };
}

function runStoryStep6(api: DagDebugAPI): TestResult {
  let st = api.getState();
  if (st.historyDepth < 2) return { ok: false, msg: 'expected deep history before close' };
  const cam2 = st.camera;
  const moved = (a: { x: number; y: number; z: number }, b: { x: number; y: number; z: number }) =>
    Math.abs(a.x - b.x) >= 1 || Math.abs(a.y - b.y) >= 1 || Math.abs(a.z - b.z) >= 1;

  if (!click('DAG Back')) return { ok: false, msg: 'first back failed' };
  st = api.getState();
  if (st.historyDepth < 1) return { ok: false, msg: 'history did not decrement after first back' };
  const cam1 = st.camera;
  if (!moved(cam2, cam1)) return { ok: false, msg: 'camera did not move after first back' };
  if (cam1.y >= cam2.y - 1) return { ok: false, msg: 'camera y should decrease when closing one layer' };

  if (!click('DAG Back')) return { ok: false, msg: 'second back failed' };
  st = api.getState();
  if (st.activeLayerId !== 'root') return { ok: false, msg: 'did not return to root after second back' };
  if (st.historyDepth !== 0) return { ok: false, msg: 'history not cleared at root' };
  if (!moved(cam1, st.camera)) return { ok: false, msg: 'camera did not move after second back' };
  if (st.camera.y >= cam1.y - 1) return { ok: false, msg: 'camera y should decrease again when returning to root' };

  const s = story();
  const processorID = String(s.processorID || '');
  const inputID = String(s.inputID || '');
  const outputID = String(s.outputID || '');
  if (!processorID || !inputID || !outputID) return { ok: false, msg: 'missing root story ids' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor at root after close' };
  st = api.getState();
  if (!st.inputNodeIDs.includes(inputID)) return { ok: false, msg: 'processor root input lost after close' };
  if (!st.outputNodeIDs.includes(outputID)) return { ok: false, msg: 'processor root output lost after close' };
  return { ok: true, msg: 'ok' };
}

function runStoryStep7(api: DagDebugAPI): TestResult {
  const s = story();
  const processorID = String(s.processorID || '');
  const inputID = String(s.inputID || '');
  const outputID = String(s.outputID || '');
  if (!outputID || !processorID || !inputID) return { ok: false, msg: 'missing root story ids' };

  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before unlink input->processor failed' };
  if (!clickNode(api, inputID)) return { ok: false, msg: 'cannot select input node for unlink output pick' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor for unlink input pick' };
  if (!click('DAG Unlink')) return { ok: false, msg: 'unlink input->processor action failed' };
  if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before unlink processor->output failed' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot select processor for second unlink output pick' };
  if (!clickNode(api, outputID)) return { ok: false, msg: 'cannot select output node for second unlink input pick' };
  if (!click('DAG Unlink')) return { ok: false, msg: 'unlink processor->output action failed' };
  if (!clickNode(api, processorID)) return { ok: false, msg: 'cannot reselect processor after unlinks' };
  let st = api.getState();
  if (st.inputNodeIDs.length !== 0) return { ok: false, msg: 'processor should have no inputs after edge deletion' };
  if (st.outputNodeIDs.length !== 0) return { ok: false, msg: 'processor should have no outputs after edge deletion' };
  if (!setName('Processor Final')) return { ok: false, msg: 'final rename failed' };
  if (api.getNodeLabel(processorID) !== 'Processor Final') return { ok: false, msg: 'final label mismatch' };
  st = api.getState();
  if (!st.camera || st.camera.y < 20) return { ok: false, msg: 'camera too low after workflow' };
  return { ok: true, msg: 'ok' };
}

const cases: Record<string, (api: DagDebugAPI) => TestResult> = {
  story_step_1: runStoryStep1,
  story_step_2: runStoryStep2,
  story_step_3: runStoryStep3,
  story_step_4: runStoryStep4,
  story_step_5: runStoryStep5,
  story_step_6: runStoryStep6,
  story_step_7: runStoryStep7,
};

(window as any).dagTestLib = {
  list: () => Object.keys(cases).sort(),
  run: (name: string): TestResult => {
    const api = requireAPI();
    if (!api) return { ok: false, msg: 'missing debug api' };
    const fn = cases[name];
    if (!fn) return { ok: false, msg: `unknown test: ${name}` };
    try {
      return fn(api);
    } catch (err) {
      return { ok: false, msg: err instanceof Error ? err.message : String(err) };
    }
  },
};

export {};
