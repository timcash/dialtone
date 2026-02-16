package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run10ThreeUserStoryDeleteAndRelabel() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step7 description:")
	fmt.Println("[THREE]   - In order to remove a node, user selects it, switches to DelN mode, and taps Action.")
	fmt.Println("[THREE]   - In order to remove a remaining edge, user selects processor, switches to DelE mode, and taps Action.")
	fmt.Println("[THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.")

	type evalResult struct {
		OK  bool   `json:"ok"`
		Msg string `json:"msg"`
	}
	var result evalResult
	if err := browser.Run(chromedp.Evaluate(`
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
			const rename = (name) => {
				const input = q('DAG Node Name');
				if (!input) return false;
				input.value = name;
				input.dispatchEvent(new Event('input', { bubbles: true }));
				return click('DAG Rename Node');
			};

			const story = window.__dagStory || {};
			const outputID = story.outputID;
			const processorID = story.processorID;
			if (!outputID || !processorID) return { ok: false, msg: 'missing root story ids' };

			if (!clickNode(outputID)) return { ok: false, msg: 'cannot select output node for deletion' };
			while (api.getState().mode !== 'remove-node') {
				if (!click('DAG Mode')) return { ok: false, msg: 'cannot switch to remove-node mode' };
			}
			if (!click('DAG Action')) return { ok: false, msg: 'remove-node action failed' };
			let st = api.getState();
			if (st.visibleNodeIDs.includes(outputID)) return { ok: false, msg: 'output node still visible after delete' };

			if (!clickNode(processorID)) return { ok: false, msg: 'cannot select processor for edge delete' };
			while (api.getState().mode !== 'remove-edge') {
				if (!click('DAG Mode')) return { ok: false, msg: 'cannot switch to remove-edge mode' };
			}
			if (!click('DAG Action')) return { ok: false, msg: 'remove-edge action failed' };
			st = api.getState();
			if (st.inputNodeIDs.length !== 0) return { ok: false, msg: 'processor should have no inputs after edge deletion' };
			if (st.outputNodeIDs.length !== 0) return { ok: false, msg: 'processor should have no outputs after output delete + edge delete' };

			if (!rename('Processor Final')) return { ok: false, msg: 'final rename failed' };
			if (api.getNodeLabel(processorID) !== 'Processor Final') return { ok: false, msg: 'final label mismatch' };

			// camera should remain zoomed out enough to keep context readable.
			st = api.getState();
			if (!st.camera || st.camera.z < 20) return { ok: false, msg: 'camera too close after workflow' };

			return { ok: true, msg: 'ok' };
		})()
	`, &result)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step7 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_8.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step7 screenshot: %w", err)
	}
	return nil
}
