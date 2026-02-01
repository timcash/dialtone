package main

import (
	"database/sql"
	"fmt"
	"os"
)

var testDB *sql.DB

func main() {
	logLine("info", "Starting nexttone integration test")

	allPassed := true
	runTest := func(name string, fn func() error) {
		logLine("test", name)
		if err := fn(); err != nil {
			logLine("fail", fmt.Sprintf("%s - %v", name, err))
			allPassed = false
		} else {
			logLine("pass", name)
		}
	}

	defer func() {
		logLine("info", "Nexttone tests completed")
		if !allPassed {
			logLine("error", "Some tests failed")
			os.Exit(1)
		}
	}()

	setupToneDir()

	runTest("Nexttone add creates tone files", TestToneAddCreatesFiles)
	runTest("Nexttone init seeds microtones", TestInitDBSeedsMicrotones)
	runTest("Nexttone subtone add creates row", TestSubtoneAddCreatesRow)
	runTest("Nexttone subtone set updates fields", TestSubtoneSetUpdatesFields)
	runTest("Nexttone list shows graph + cursor", TestListShowsGraph)
	runTest("Nexttone requires signature", TestNextRequiresSignature)
	runTest("Nexttone sign advances microtone", TestSignAdvancesMicrotone)
	runTest("Nexttone loops over subtones", TestLoopOverSubtones)
	runTest("Nexttone LLM workflow guidance", TestLLMWorkflowGuidance)

	cleanupToneDir()
	fmt.Println()
}
