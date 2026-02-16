package main

import (
	"fmt"
)

func Run05ThreeUserStoryNestAndOpenLayer() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step3 description:")
	fmt.Println("[THREE]   - In order to create a nested layer, the user selects processor and taps Nest.")
	fmt.Println("[THREE]   - After opening the layer, user builds nested nodes using Add, then links them explicitly.")
	fmt.Println("[THREE]   - Camera/layout expectation: nested layer anchors to parent x/z and elevates on +y; open-layer camera tracks that elevation.")

	if err := captureStoryShot(browser, "test_step_4_pre.png"); err != nil {
		return fmt.Errorf("capture story step3 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_3"); err != nil {
		return fmt.Errorf("story step3 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_4.png"); err != nil {
		return fmt.Errorf("capture story step3 screenshot: %w", err)
	}
	return nil
}
