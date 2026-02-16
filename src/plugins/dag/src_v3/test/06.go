package main

import (
	"fmt"
)

func Run06ThreeUserStoryRenameAndCloseLayer() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step4 description:")
	fmt.Println("[THREE]   - In order to change labels, the user selects node, types name in bottom textbox, and taps Rename.")
	fmt.Println("[THREE]   - In order to close an opened layer, the user taps Back once to return to root.")
	fmt.Println("[THREE]   - Camera expectation: layer close moves camera to the parent node and updates history to zero.")

	if err := captureStoryShot(browser, "test_step_5_pre.png"); err != nil {
		return fmt.Errorf("capture story step4 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_4"); err != nil {
		return fmt.Errorf("story step4 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_5.png"); err != nil {
		return fmt.Errorf("capture story step4 screenshot: %w", err)
	}
	return nil
}
