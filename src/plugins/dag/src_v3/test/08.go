package main

import (
	"fmt"
)

func Run08ThreeUserStoryDeepNestedBuild() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step5 description:")
	fmt.Println("[THREE]   - In order to open an existing nested layer, user selects processor and taps Nest.")
	fmt.Println("[THREE]   - In order to create second-level nested layer, user selects nested node and taps Nest.")
	fmt.Println("[THREE]   - Camera/layout expectation: each deeper opened nested layer stacks higher on +y and camera y rises with depth.")

	if err := captureStoryShot(browser, "test_step_6_pre.png"); err != nil {
		return fmt.Errorf("capture story step5 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_5"); err != nil {
		return fmt.Errorf("story step5 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_6.png"); err != nil {
		return fmt.Errorf("capture story step5 screenshot: %w", err)
	}
	return nil
}
