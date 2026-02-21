package chrome

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenInRegularChrome opens the given URL in the system's default Chrome installation
// without any special debugging flags or profiles.
func OpenInRegularChrome(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Google Chrome" to open location %q`, url))
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", "chrome", url)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return cmd.Run()
}
