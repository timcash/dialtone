package main

import (
	"github.com/chromedp/chromedp"
)

func Run11HeroSectionValidation() error {
	session, err := ensureSharedBrowser(false)
	if err != nil {
		return err
	}

	if err := session.Run(chromedp.Tasks{
		chromedp.WaitVisible("[aria-label='Hero Section']", chromedp.ByQuery),
		chromedp.WaitVisible("[aria-label='Hero Canvas']", chromedp.ByQuery),
	}); err != nil {
		return err
	}

	shot, err := screenshotPath("test_step_1.png")
	if err != nil {
		return err
	}
	return session.CaptureScreenshot(shot)
}
