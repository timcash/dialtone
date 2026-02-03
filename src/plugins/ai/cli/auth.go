package cli

import (
	"dialtone/cli/src/core/logger"
	"os"

	"github.com/joho/godotenv"
)

// RunAIAuth provides guidance on authenticating with the Gemini CLI
func RunAIAuth(args []string) {
	logger.LogInfo("AI Plugin: Starting Gemini CLI authentication diagnostic...")

	// Load env/.env
	if err := godotenv.Load("env/.env"); err != nil {
		logger.LogDebug("AI Plugin: No env/.env file found")
	}

	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey != "" {
		logger.LogInfo("AI Plugin: Authentication key (GOOGLE_API_KEY) found in env/.env")
	} else {
		logger.LogInfo("AI Plugin: No GOOGLE_API_KEY found in env/.env.")
	}

	logger.LogInfo("")
	logger.LogInfo("--- Authentication Guidance ---")
	logger.LogInfo("1. Get a Google API key from AI Studio: https://aistudio.google.com/app/apikey")
	logger.LogInfo("2. Add it to your env/.env file in the project root:")
	logger.LogInfo("   GOOGLE_API_KEY=your_actual_key_here")
	logger.LogInfo("3. The AI plugin will automatically use this key.")
	logger.LogInfo("-------------------------------")
}
