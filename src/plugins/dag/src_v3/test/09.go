package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run09ThreeUserStoryDeepUndiveHistory() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step6 description:")
	fmt.Println("[THREE]   - In order to unwind nested history, user taps Back repeatedly.")
	fmt.Println("[THREE]   - Each back step must reduce history depth and reposition camera to the new current layer.")
	fmt.Println("[THREE]   - Final expectation: root layer visible with processor input/output context intact.")

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
			const moved = (a, b) =>
				Math.abs(a.x - b.x) >= 1 || Math.abs(a.y - b.y) >= 1 || Math.abs(a.z - b.z) >= 1;

			let st = api.getState();
			if (st.historyDepth < 2) return { ok: false, msg: 'expected deep history before undive' };
			const cam2 = st.camera;

			if (!click('DAG Back')) return { ok: false, msg: 'first back failed' };
			st = api.getState();
			if (st.historyDepth < 1) return { ok: false, msg: 'history did not decrement after first back' };
			const cam1 = st.camera;
			if (!moved(cam2, cam1)) return { ok: false, msg: 'camera did not move after first back' };

			if (!click('DAG Back')) return { ok: false, msg: 'second back failed' };
			st = api.getState();
			if (st.activeLayerId !== 'root') return { ok: false, msg: 'did not return to root after second back' };
			if (st.historyDepth !== 0) return { ok: false, msg: 'history not cleared at root' };
			if (!moved(cam1, st.camera)) return { ok: false, msg: 'camera did not move after second back' };

			// Validate processor still has root IO after multi-layer traversal.
			const story = window.__dagStory || {};
			const processorID = story.processorID;
			const inputID = story.inputID;
			const outputID = story.outputID;
			if (!processorID || !inputID || !outputID) return { ok: false, msg: 'missing root story ids' };

			const p = api.getProjectedPoint(processorID);
			if (!p || !p.ok) return { ok: false, msg: 'processor projection missing at root' };
			const canvas = q('Three Canvas');
			canvas.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, clientX: p.x, clientY: p.y, view: window }));
			st = api.getState();
			if (st.selectedNodeId !== processorID) return { ok: false, msg: 'cannot select processor at root after undive' };
			if (!st.inputNodeIDs.includes(inputID)) return { ok: false, msg: 'processor root input lost after undive' };
			if (!st.outputNodeIDs.includes(outputID)) return { ok: false, msg: 'processor root output lost after undive' };

			return { ok: true, msg: 'ok' };
		})()
	`, &result)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step6 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_7.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step6 screenshot: %w", err)
	}
	return nil
}
