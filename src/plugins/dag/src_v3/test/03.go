package main

import (
	"fmt"
	"time"

	test_v2 "dialtone/cli/src/libs/test_v2"
	"github.com/chromedp/chromedp"
)

func Run03ThreeUserStoryStartEmpty() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step1 description:")
	fmt.Println("[THREE]   - In order to create a new node, the user taps Add.")
	fmt.Println("[THREE]   - The user starts from an empty DAG in root layer and expects one selected node after add.")
	fmt.Println("[THREE]   - Camera expectation: zoomed-out root framing with room for upcoming input/output nodes.")

	if err := browser.Run(chromedp.Tasks{
		chromedp.Navigate("http://127.0.0.1:8080/#three"),
		test_v2.WaitForAriaLabel("Three Canvas"),
		test_v2.WaitForAriaLabelAttrEquals("Three Canvas", "data-ready", "true", 3*time.Second),
		test_v2.WaitForAriaLabel("DAG Back"),
		test_v2.WaitForAriaLabel("DAG Add"),
		test_v2.WaitForAriaLabel("DAG Connect"),
		test_v2.WaitForAriaLabel("DAG Unlink"),
		test_v2.WaitForAriaLabel("DAG Nest"),
		test_v2.WaitForAriaLabel("DAG Clear Picks"),
		test_v2.WaitForAriaLabel("DAG Label Input"),
	}); err != nil {
		return err
	}
	if err := captureStoryShot(browser, "test_step_2_pre.png"); err != nil {
		return fmt.Errorf("capture story step1 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_1"); err != nil {
		return fmt.Errorf("story step1 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_2.png"); err != nil {
		return fmt.Errorf("capture story step1 screenshot: %w", err)
	}
	return nil
}
