package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run08ThreeUserStoryDeepNestedBuild() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step5 description:")
	fmt.Println("[THREE]   - In order to dive into existing nested layer, user selects processor and taps Nest.")
	fmt.Println("[THREE]   - In order to create second-level nested layer, user selects nested node and taps Nest.")
	fmt.Println("[THREE]   - Camera expectation: each deeper dive keeps active layer centered and still comfortably zoomed out.")

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

			const story = window.__dagStory || {};
			const processorID = story.processorID;
			const nestedB = story.nestedB;
			if (!processorID || !nestedB) return { ok: false, msg: 'missing story ids for deep nesting' };

			// Dive root -> processor nested layer.
			if (!clickNode(processorID)) return { ok: false, msg: 'select processor failed' };
			if (!click('DAG Nest')) return { ok: false, msg: 'enter processor nested layer failed' };
			if (api.getState().historyDepth < 1) return { ok: false, msg: 'history depth missing after first dive' };

			// Create second-level nest on nestedB.
			if (!clickNode(nestedB)) return { ok: false, msg: 'select nestedB failed' };
			if (!click('DAG Nest')) return { ok: false, msg: 'create + enter second-level nested layer failed' };
			let st = api.getState();
			if (st.historyDepth < 2) return { ok: false, msg: 'history depth missing after second dive' };
			const level2LayerID = st.activeLayerId;
			if (!level2LayerID || level2LayerID === 'root') return { ok: false, msg: 'missing level2 active layer id' };

			// Build two nodes and connect in deepest layer.
			if (!click('DAG Add')) return { ok: false, msg: 'add level2 node A failed' };
			st = api.getState();
			const level2A = st.lastCreatedNodeId;
			if (!level2A) return { ok: false, msg: 'missing level2A id' };

			if (!click('DAG Add')) return { ok: false, msg: 'add level2 node B failed' };
			st = api.getState();
			const level2B = st.lastCreatedNodeId;
			if (!level2B || level2B === level2A) return { ok: false, msg: 'missing level2B id' };

			if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before level2 connect failed' };
			if (!clickNode(level2A)) return { ok: false, msg: 'select level2A failed' };
			if (!clickNode(level2B)) return { ok: false, msg: 'select level2B failed' };
			if (!click('DAG Connect')) return { ok: false, msg: 'apply level2 connect failed' };
			st = api.getState();
			if (!st.inputNodeIDs.includes(level2A)) return { ok: false, msg: 'level2 edge missing' };

			story.level2LayerID = level2LayerID;
			story.level2A = level2A;
			story.level2B = level2B;
			story.level2Camera = st.camera;
			window.__dagStory = story;
			return { ok: true, msg: 'ok' };
		})()
	`, &result)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step5 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_6.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step5 screenshot: %w", err)
	}
	return nil
}
