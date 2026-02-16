package main

import (
	"fmt"
)

func Run10ThreeUserStoryUnlinkAndRelabel() error {
	browser, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}
	fmt.Println("[THREE] story step7 description:")
	fmt.Println("[THREE]   - In order to remove edges, user selects output/input nodes and taps Unlink.")
	fmt.Println("[THREE]   - User clears selections between unlink actions.")
	fmt.Println("[THREE]   - User then renames processor again and expects camera to stay zoomed-out for full root readability.")

	if err := captureStoryShot(browser, "test_step_8_pre.png"); err != nil {
		return fmt.Errorf("capture story step7 pre screenshot: %w", err)
	}
	if err := runThreeCase(browser, "story_step_7"); err != nil {
		return fmt.Errorf("story step7 failed: %w", err)
	}
	if err := captureStoryShot(browser, "test_step_8.png"); err != nil {
		return fmt.Errorf("capture story step7 screenshot: %w", err)
	}
	return nil
}
