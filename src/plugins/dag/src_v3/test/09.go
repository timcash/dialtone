package main

import (
	"fmt"
)

func Run09ThreeUserStoryDeepCloseLayerHistory() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step6 description:")
	fmt.Println("[THREE]   - In order to close opened nested layers, user taps Back repeatedly.")
	fmt.Println("[THREE]   - Each close action must reduce history depth and lower camera y as the stack unwinds.")
	fmt.Println("[THREE]   - Final expectation: root layer visible with processor input/output context intact.")

	if err := captureStoryShot(browser, "test_step_7_pre.png"); err != nil {
		return fmt.Errorf("capture story step6 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_6"); err != nil {
		return fmt.Errorf("story step6 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_7.png"); err != nil {
		return fmt.Errorf("capture story step6 screenshot: %w", err)
	}
	return nil
}
