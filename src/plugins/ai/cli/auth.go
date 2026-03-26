package cli

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
	"os"
)

// RunAIAuth provides guidance on authenticating with the Gemini CLI
func RunAIAuth(args []string) {
	logs.Info("AI Plugin: Starting Gemini CLI authentication diagnostic...")

	rt, err := configv1.ResolveRuntime("")
	if err == nil {
		if loadErr := configv1.LoadEnvFile(rt); loadErr != nil {
			logs.Debug("AI Plugin: Failed loading env/dialtone.json: %v", loadErr)
		}
	}

	googleKey := os.Getenv("GOOGLE_API_KEY")
	if googleKey != "" {
		logs.Info("AI Plugin: Authentication key (GOOGLE_API_KEY) found in env/dialtone.json")
	} else {
		logs.Info("AI Plugin: No GOOGLE_API_KEY found in env/dialtone.json.")
	}

	logs.Info("")
	logs.Info("--- Authentication Guidance ---")
	logs.Info("1. Get a Google API key from AI Studio: https://aistudio.google.com/app/apikey")
	logs.Info("2. Add it to env/dialtone.json in the project root:")
	logs.Info("   \"GOOGLE_API_KEY\": \"your_actual_key_here\"")
	logs.Info("3. The AI plugin will automatically use this key.")
	logs.Info("-------------------------------")
}
