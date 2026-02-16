package main

import (
	"fmt"

	test_v2 "dialtone/cli/src/libs/test_v2"
)

func Run04ThreeUserStoryBuildIO() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step2 description:")
	fmt.Println("[THREE]   - In order to add output, the user selects processor and taps Add.")
	fmt.Println("[THREE]   - Add creates nodes only; user selects output=processor and input=output before tapping Link.")
	fmt.Println("[THREE]   - In order to add input, the user clears selection, taps Add, then selects output=input and input=processor before tapping Link.")
	fmt.Println("[THREE]   - Camera expectation: root layer remains fully readable while adding and linking nodes.")

	if err := browser.Run(test_v2.WaitForAriaLabel("Three Canvas")); err != nil {
		return err
	}
	if err := captureStoryShot(browser, "test_step_3_pre.png"); err != nil {
		return fmt.Errorf("capture story step2 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_2"); err != nil {
		return fmt.Errorf("story step2 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_3.png"); err != nil {
		return fmt.Errorf("capture story step2 screenshot: %w", err)
	}
	return nil
}
