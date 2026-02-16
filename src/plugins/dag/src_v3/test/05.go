package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

func Run05ThreeUserStoryNestAndDive() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step3 description:")
	fmt.Println("[THREE]   - In order to create a nested layer, the user selects processor and taps Nest.")
	fmt.Println("[THREE]   - After dive, user builds nested nodes using Add, then links them explicitly.")
	fmt.Println("[THREE]   - Camera expectation: on dive, framing shifts to nested layer and history depth increments.")

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
			if (!processorID) return { ok: false, msg: 'missing processor id' };

			const rootBeforeDive = api.getState().camera;
			if (!clickNode(processorID)) return { ok: false, msg: 'select processor failed' };
			if (!click('DAG Nest')) return { ok: false, msg: 'nest action failed' };

			let st = api.getState();
			if (!st.activeLayerId || st.activeLayerId === 'root') return { ok: false, msg: 'did not dive into nested layer' };
			if (st.historyDepth < 1) return { ok: false, msg: 'history depth not increased on dive' };
			const nestedLayerID = st.activeLayerId;
			story.nestedLayerID = nestedLayerID;
			story.rootCameraBeforeDive = rootBeforeDive;
			story.nestedCameraAfterDive = st.camera;

			// First nested node
			if (!click('DAG Add')) return { ok: false, msg: 'nested add node A failed' };
			st = api.getState();
			const nestedA = st.lastCreatedNodeId;
			if (!nestedA) return { ok: false, msg: 'nested node A id missing' };

			// Second nested node
			if (!click('DAG Add')) return { ok: false, msg: 'nested add node B failed' };
			st = api.getState();
			const nestedB = st.lastCreatedNodeId;
			if (!nestedB || nestedB === nestedA) return { ok: false, msg: 'nested node B id missing' };

			// Link nestedA -> nestedB.
			if (!click('DAG Clear Picks')) return { ok: false, msg: 'clear picks before nested link failed' };
			if (!clickNode(nestedA)) return { ok: false, msg: 'select nestedA for link source failed' };
			if (!clickNode(nestedB)) return { ok: false, msg: 'select nestedB for link target failed' };
			if (!click('DAG Connect')) return { ok: false, msg: 'connect nestedA->nestedB failed' };

			// Validate nested edge exists by selecting first nested node.
			if (!clickNode(nestedA)) return { ok: false, msg: 'reselect nestedA failed' };
			st = api.getState();
			if (!st.outputNodeIDs.includes(nestedB)) return { ok: false, msg: 'nestedA missing output to nestedB' };

			story.nestedA = nestedA;
			story.nestedB = nestedB;
			window.__dagStory = story;
			return { ok: true, msg: 'ok' };
		})()
	`, &result)); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step3 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_4.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step3 screenshot: %w", err)
	}
	return nil
}
