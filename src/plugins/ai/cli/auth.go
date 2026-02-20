package cli

import (
	"dialtone/dev/plugins/logs/src_v1/go"
	"os"

	"github.com/joho/godotenv"
)

// RunAIAuth provides guidance on authenticating with the Gemini CLI
func RunAIAuth(args []string) {
	logs.Info("AI Plugin: Starting Gemini CLI authentication diagnostic...")

	// Load env/.env
	if err := godotenv.Load("env/.env"); err != nil {
		logs.Debug("AI Plugin: No env/.env file found")
	}

	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey != "" {
		logs.Info("AI Plugin: Authentication key (GOOGLE_API_KEY) found in env/.env")
	} else {
		logs.Info("AI Plugin: No GOOGLE_API_KEY found in env/.env.")
	}

	logs.Info("")
	logs.Info("--- Authentication Guidance ---")
	logs.Info("1. Get a Google API key from AI Studio: https://aistudio.google.com/app/apikey")
	logs.Info("2. Add it to your env/.env file in the project root:")
	logs.Info("   GOOGLE_API_KEY=your_actual_key_here")
	logs.Info("3. The AI plugin will automatically use this key.")
	logs.Info("-------------------------------")
}
