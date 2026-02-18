package main

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

func Run02LogSectionLoad(ctx *testCtx) (string, error) {
	sess, err := ctx.ensureSharedBrowser()
	if err != nil {
		return "", err
	}
	allocCtx, cancel := context.WithTimeout(sess.Context(), 15*time.Second)
	defer cancel()

	var visible string
	err = chromedp.Run(allocCtx,
		chromedp.Navigate(ctx.baseURL+"/#logs-log-xterm"),
		chromedp.WaitVisible(`#logs-log-xterm`, chromedp.ByID),
		chromedp.AttributeValue(`#logs-log-xterm`, "data-active", &visible, nil),
	)
	if err != nil {
		return "", err
	}
	return "Log section loaded and visible.", nil
}
