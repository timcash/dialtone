package cli

import (
	"dialtone/cli/src/core/logger"
	"strings"
)

// RunGemini handles the --gemini flag functionality
func RunGemini(args []string) {
	if len(args) == 0 {
		logger.LogInfo("Gemini: Please provide a prompt. Usage: dialtone ai --gemini \"prompt\"")
		return
	}

	prompt := strings.Join(args, " ")
	logger.LogInfo("Gemini: Processing prompt '%s'...", prompt)

	// TODO: Integrate with actual Gemini API using API key from env
	// For now, this is a stub verification used by the test command
	logger.LogInfo("Gemini: [STUB] Response to '%s'", prompt)
}
