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
	fmt.Println("[THREE]   - In order to add output, the user selects processor and taps Action in Add mode.")
	fmt.Println("[THREE]   - In order to add input, the user clears selection, taps Action in Add mode, then links input->processor in Connect mode.")
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
				const story = (window.__dagStory = window.__dagStory || {});
				const processorID = story.processorID;
				if (!processorID) return { ok: false, msg: 'missing processor id from step1' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor' };
				while (api.getState().mode !== 'add') {
					if (!click('DAG Mode')) return { ok: false, msg: 'cannot switch to add mode' };
				}

				// Add output from processor (processor -> output)
				if (!click('DAG Action')) return { ok: false, msg: 'add output failed' };
				let st = api.getState();
				const outputID = st.lastCreatedNodeId;
				if (!outputID || outputID === processorID) return { ok: false, msg: 'missing output node id' };

				// Add a standalone input by clearing selection, then add.
				const canvas = q('Three Canvas');
				canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: 8, clientY: 8, view: window }));
				if (api.getState().selectedNodeId !== '') return { ok: false, msg: 'failed to clear selection' };
				if (!click('DAG Action')) return { ok: false, msg: 'add input failed' };
				st = api.getState();
				const inputID = st.lastCreatedNodeId;
				if (!inputID || inputID === outputID || inputID === processorID) return { ok: false, msg: 'missing input node id' };

				// Connect input -> processor using connect mode.
				while (api.getState().mode !== 'connect') {
					if (!click('DAG Mode')) return { ok: false, msg: 'cannot switch to connect mode' };
				}
				if (!clickNode(inputID)) return { ok: false, msg: 'cannot select input node' };
				if (!click('DAG Action')) return { ok: false, msg: 'connect arm failed' };
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor for connect target' };
				if (!click('DAG Action')) return { ok: false, msg: 'connect apply failed' };

				// Validate processor has both input and output.
				if (!clickNode(processorID)) return { ok: false, msg: 'cannot reselect processor' };
				st = api.getState();
				if (!st.inputNodeIDs.includes(inputID)) return { ok: false, msg: 'processor missing input edge' };
				if (!st.outputNodeIDs.includes(outputID)) return { ok: false, msg: 'processor missing output edge' };

				story.inputID = inputID;
				story.outputID = outputID;
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
