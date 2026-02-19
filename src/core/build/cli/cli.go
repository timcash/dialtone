package cli

import (
	"dialtone/dev/core/build"
	"fmt"
)

// Run handles the 'build' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	if len(args) > 0 && args[0] == "test" {
		runBuildTests()
		return
	}

	build.RunBuild(args)
}

func printUsage() {
	fmt.Println("Usage: dialtone build [options]")
	fmt.Println()
	fmt.Println("Build the Dialtone binary and web UI for deployment.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --local        Build natively on the local system (uses DIALTONE_ENV if available)")
	fmt.Println("  --full         Full rebuild: Web UI + local CLI + ARM64 binary")
	fmt.Println("  --remote       Build on remote robot via SSH (requires configured .env)")
	fmt.Println("  --podman       Force build using Podman container")
	fmt.Println("  --linux-arm    Cross-compile for 32-bit Linux ARM (Raspberry Pi Zero/3/4/5)")
	fmt.Println("  --linux-arm64  Cross-compile for 64-bit Linux ARM (Raspberry Pi 3/4/5)")
	fmt.Println("  --linux-amd64  Cross-compile for 64-bit Linux x86 (amd64)")
	fmt.Println("  --builder      Build the dialtone-builder image for faster ARM builds")
	fmt.Println("  test           Run build system integration test")
	fmt.Println("  help           Show help for build command")
	fmt.Println("  --help         Show help for build command")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  dialtone build              # Build web UI + binary (Podman or local)")
	fmt.Println("  dialtone build --local      # Build web UI + native binary")
	fmt.Println("  dialtone build --podman     # Force Podman build for ARM64")
	fmt.Println("  dialtone build --linux-arm  # Cross-compile for 32-bit ARM")
	fmt.Println("  dialtone build test         # Run build system test")
}
