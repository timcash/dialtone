package main

import "fmt"

func Run03Finalize(ctx *testCtx) (string, error) {
	testLines := lineCount(ctx.testLogPath)
	errorLines := lineCount(ctx.errorLog)
	if testLines < 2 {
		return "", fmt.Errorf("expected at least 2 lines in %s, got %d", ctx.testLogPath, testLines)
	}
	if errorLines < 1 {
		return "", fmt.Errorf("expected at least 1 line in %s, got %d", ctx.errorLog, errorLines)
	}
	return fmt.Sprintf("Artifacts ready: test.log=%d lines, error.log=%d lines.", testLines, errorLines), nil
}
