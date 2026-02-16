package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run03ThreeUserStoryStartEmpty() error {
	browser, err := ensureSharedBrowser(true)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step1 description:")
	fmt.Println("[THREE]   - In order to create a new node, the user taps Add.")
	fmt.Println("[THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.")
	fmt.Println("[THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.")

	type evalResult struct {
		OK  bool   `json:"ok"`
		Msg string `json:"msg"`
	}
	var result evalResult
	if err := browser.Run(chromedp.Tasks{
		chromedp.Navigate("http://127.0.0.1:8080/#three"),
		test_v2.WaitForAriaLabel("Three Canvas"),
		test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-ready", "true", 3*time.Second),
		test_v2.WaitForAriaLabel("DAG Back"),
		test_v2.WaitForAriaLabel("DAG Add"),
		test_v2.WaitForAriaLabel("DAG Pick Output"),
		test_v2.WaitForAriaLabel("DAG Pick Input"),
		test_v2.WaitForAriaLabel("DAG Connect"),
		test_v2.WaitForAriaLabel("DAG Unlink"),
		test_v2.WaitForAriaLabel("DAG Nest"),
		test_v2.WaitForAriaLabel("DAG Delete Node"),
		test_v2.WaitForAriaLabel("DAG Clear Picks"),
		test_v2.WaitForAriaLabel("DAG Label Input"),
		chromedp.Evaluate(`
			(() => {
				const api = window.dagHitTestDebug;
				if (!api || typeof api.getState !== 'function') return { ok: false, msg: 'missing debug api' };
				const q = (name) => document.querySelector("[aria-label='" + name + "']");
				const click = (name) => {
					const el = q(name);
					if (!el) return false;
					el.click();
					return true;
				};

				const initial = api.getState();
				if (!initial || initial.activeLayerId !== 'root') return { ok: false, msg: 'initial root layer missing' };
				if (initial.visibleNodeIDs.length !== 0) return { ok: false, msg: 'expected empty dag at start' };

				if (!click('DAG Add')) return { ok: false, msg: 'add action failed' };

				const afterAdd = api.getState();
				if (afterAdd.visibleNodeIDs.length !== 1) return { ok: false, msg: 'first node not created' };
				const processorID = afterAdd.lastCreatedNodeId;
				if (!processorID) return { ok: false, msg: 'missing processor node id' };
				if (afterAdd.selectedNodeId !== processorID) return { ok: false, msg: 'new node not selected' };

				const store = (window.__dagStory = window.__dagStory || {});
				store.processorID = processorID;
				store.rootCameraBeforeDive = afterAdd.camera;
				return { ok: true, msg: 'ok' };
			})()
		`, &result),
	}); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("story step1 failed: %s", result.Msg)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	shot := filepath.Join(repoRoot, "src", "plugins", "dag", "src_v3", "screenshots", "test_step_2.png")
	if err := browser.CaptureScreenshot(shot); err != nil {
		return fmt.Errorf("capture story step1 screenshot: %w", err)
	}
	return nil
}
