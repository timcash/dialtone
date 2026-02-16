package main

import (
	"fmt"
	"os"
	"path/filepath"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run04ThreeUserStoryBuildIO() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step2 description:")
	fmt.Println("[THREE]   - In order to add output, the user selects processor and taps Add.")
	fmt.Println("[THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.")
	fmt.Println("[THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.")
	fmt.Println("[THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.")

	type evalResult struct {
		OK  bool   `json:"ok"`
		Msg string `json:"msg"`
	}
	var result evalResult
	if err := browser.Run(chromedp.Tasks{
		test_v2.WaitForAriaLabel("Three Canvas"),
		chromedp.Evaluate(`
			(() => {
				const api = window.dagHitTestDebug;
				if (!api) return { ok: false, msg: 'missing debug api' };
				const q = (name) => document.querySelector("[aria-label='" + name + "']");
				const click = (name) => {
					const el = q(name);
					if (!el) return false;
					el.click();
					return true;
				};
				const clickNode = (nodeId) => {
					const p = api.getProjectedPoint(nodeId);
					if (!p || !p.ok) return false;
					const canvas = q('Three Canvas');
					canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: p.x, clientY: p.y, view: window }));
					return api.getState().selectedNodeId === nodeId;
				};
				const historyValue = (name) => {
					const el = q(name);
					return el ? String(el.textContent || '').trim() : '';
				};
				const nodeColorHex = (nodeId) => {
					const info = api.getNodeColorHex(nodeId);
					return info && info.ok ? info.colorHex : -1;
				};
				const story = (window.__dagStory = window.__dagStory || {});
				const processorID = story.processorID;
				if (!processorID) return { ok: false, msg: 'missing processor id from step1' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor' };

				// Add output from processor (processor -> output)
				if (!click('DAG Add')) return { ok: false, msg: 'add output failed' };
				let st = api.getState();
				const outputID = st.lastCreatedNodeId;
				if (!outputID || outputID === processorID) return { ok: false, msg: 'missing output node id' };
				if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before connect processor->output failed' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor for output link source' };
				if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'history item1 did not show processor' };
				if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be none after first selection' };
				if (nodeColorHex(processorID) !== 0x7dd3fc) return { ok: false, msg: 'most recent node color should be light blue' };
				if (!clickNode(outputID)) return { ok: false, msg: 'cannot select output node for output link target' };
				if (historyValue('DAG Node History Item 1') !== outputID) return { ok: false, msg: 'history item1 did not show output node after second selection' };
				if (historyValue('DAG Node History Item 2') !== processorID) return { ok: false, msg: 'history item2 did not show processor after second selection' };
				if (nodeColorHex(outputID) !== 0x7dd3fc) return { ok: false, msg: 'most recent node color should stay light blue after second selection' };
				if (nodeColorHex(processorID) !== 0x2b78ff) return { ok: false, msg: 'second most recent node color should be blue' };
				if (!click('DAG Connect')) return { ok: false, msg: 'connect processor->output failed' };

				// Add a standalone input by clearing selection, then add.
				const canvas = q('Three Canvas');
				canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: 8, clientY: 8, view: window }));
				if (api.getState().selectedNodeId !== '') return { ok: false, msg: 'failed to clear selection' };
				if (!click('DAG Add')) return { ok: false, msg: 'add input failed' };
				st = api.getState();
				const inputID = st.lastCreatedNodeId;
				if (!inputID || inputID === outputID || inputID === processorID) return { ok: false, msg: 'missing input node id' };

				// Connect input -> processor (select output then input, then link).
				if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before connect failed' };
				if (!clickNode(inputID)) return { ok: false, msg: 'cannot select input node' };
				if (historyValue('DAG Node History Item 1') !== inputID) return { ok: false, msg: 'history item1 did not show input node after first selection' };
				if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be none before second selection' };
				if (nodeColorHex(inputID) !== 0x7dd3fc) return { ok: false, msg: 'new most recent node should be light blue' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor for connect target' };
				if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'history item1 did not show processor after second selection' };
				if (historyValue('DAG Node History Item 2') !== inputID) return { ok: false, msg: 'history item2 did not show input node after second selection' };
				if (nodeColorHex(processorID) !== 0x7dd3fc) return { ok: false, msg: 'processor should be light blue when most recent' };
				if (nodeColorHex(inputID) !== 0x2b78ff) return { ok: false, msg: 'input node should be blue when second most recent' };
				if (nodeColorHex(outputID) !== 0x5b6873) return { ok: false, msg: 'older nodes should be gray' };
				if (!click('DAG Connect')) return { ok: false, msg: 'connect apply failed' };

				// Validate processor has both input and output.
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot reselect processor' };
				st = api.getState();
				if (!st.inputNodeIDs.includes(inputID)) return { ok: false, msg: 'processor missing input edge' };
				if (!st.outputNodeIDs.includes(outputID)) return { ok: false, msg: 'processor missing output edge' };
				const inputTx = api.getNodeTransform(inputID);
				const processorTx = api.getNodeTransform(processorID);
				if (!inputTx.ok || !processorTx.ok) return { ok: false, msg: 'missing node transforms for rank checks' };
				if (processorTx.rank < inputTx.rank + 1) return { ok: false, msg: 'rank rule violated: input node did not move above highest input rank' };

				story.inputID = inputID;
				story.outputID = outputID;

				// Leave the two most recent selections visible in history for the step screenshot.
				if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before screenshot setup failed' };
				if (!clickNode(inputID)) return { ok: false, msg: 'cannot select input for screenshot first pick' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor for screenshot second pick' };
				if (historyValue('DAG Node History Item 1') !== processorID) return { ok: false, msg: 'screenshot history item1 mismatch' };
				if (historyValue('DAG Node History Item 2') !== inputID) return { ok: false, msg: 'screenshot history item2 mismatch' };
				const camBeforeBack = api.getState().camera;
				if (!click('DAG Back')) return { ok: false, msg: 'back action for history pop failed' };
				const afterBack = api.getState();
				if (afterBack.selectedNodeId !== inputID) return { ok: false, msg: 'back should select previous node from history' };
				if (historyValue('DAG Node History Item 1') !== inputID) return { ok: false, msg: 'history item1 should pop to previous node after back' };
				if (historyValue('DAG Node History Item 2') !== 'none') return { ok: false, msg: 'history item2 should be cleared after single back pop' };
				if (Math.abs(camBeforeBack.x - afterBack.camera.x) < 0.5 &&
					Math.abs(camBeforeBack.y - afterBack.camera.y) < 0.5 &&
					Math.abs(camBeforeBack.z - afterBack.camera.z) < 0.5) {
					return { ok: false, msg: 'camera should move to previous selected node on back' };
				}
				if (!api.setCameraView('iso')) return { ok: false, msg: 'failed to reset camera after back assertion' };
				return { ok: true, msg: 'ok' };
			})()
		`, &result),
	}); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step2 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_3.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step2 screenshot: %w", err)
	}
	return nil
}
