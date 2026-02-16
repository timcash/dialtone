package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run06ThreeUserStoryRenameAndUndive() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step4 description:")
	fmt.Println("[THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.")
	fmt.Println("[THREE]   - In order to undive, the user taps Back once to return to root.")
	fmt.Println("[THREE]   - Camera expectation: back navigation re-centers root layer and updates history to zero.")

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
			const setName = (text) => {
				const input = q('DAG Node Name');
				if (!input) return false;
				input.value = text;
				input.dispatchEvent(new Event('input', { bubbles: true }));
				return click('DAG Rename Node');
			};

			const story = window.__dagStory || {};
			const nestedA = story.nestedA;
			const processorID = story.processorID;
			if (!nestedA || !processorID) return { ok: false, msg: 'missing story ids' };
			if (api.getState().activeLayerId === 'root') return { ok: false, msg: 'expected to start in nested layer' };

			// Rename nested node and validate label value.
			if (!clickNode(nestedA)) return { ok: false, msg: 'select nestedA failed' };
			if (!setName('Nested Input')) return { ok: false, msg: 'rename nestedA action failed' };
			if (api.getNodeLabel(nestedA) !== 'Nested Input') return { ok: false, msg: 'nested label did not update' };

			// Toggle labels on so renamed text is visible in-scene.
			while (api.getState().mode !== 'labels') {
				if (!click('DAG Mode')) return { ok: false, msg: 'cannot switch to labels mode' };
			}
			if (!click('DAG Action')) return { ok: false, msg: 'enable labels failed' };
			if (!api.getState().labelsVisible) return { ok: false, msg: 'labels did not turn on' };

			// Undive back to root and verify camera and history move.
			const nestedCamera = api.getState().camera;
			if (!click('DAG Back')) return { ok: false, msg: 'back action failed' };
			const rootState = api.getState();
			if (rootState.activeLayerId !== 'root') return { ok: false, msg: 'did not return to root layer' };
			if (rootState.historyDepth !== 0) return { ok: false, msg: 'history not cleared after undive' };
			const camMoved =
				Math.abs(rootState.camera.x - nestedCamera.x) >= 1 ||
				Math.abs(rootState.camera.y - nestedCamera.y) >= 1 ||
				Math.abs(rootState.camera.z - nestedCamera.z) >= 1;
			if (!camMoved) return { ok: false, msg: 'camera did not move on undive' };

			// Rename processor on root layer and verify label.
			if (!clickNode(processorID)) return { ok: false, msg: 'select processor after undive failed' };
			if (!setName('Processor')) return { ok: false, msg: 'rename processor action failed' };
			if (api.getNodeLabel(processorID) !== 'Processor') return { ok: false, msg: 'processor label did not update' };
			if (api.getState().selectedNodeLabel !== 'Processor') return { ok: false, msg: 'selected node label state mismatch' };

			return { ok: true, msg: 'ok' };
		})()
	`, &result)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step4 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_5.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step4 screenshot: %w", err)
	}
	return nil
}
