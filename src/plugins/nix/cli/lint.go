package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RunLint() error {
	fmt.Println(">> [NIX] Lint: starting...")
	cwd, _ := os.Getwd()
	nixDir := filepath.Join(cwd, "src", "plugins", "nix")

	// 1. Lint Go Code (use dialtone go toolchain)
	fmt.Println(">> [NIX] Lint: checking Go code...")
	cmd := exec.Command("./dialtone.sh", "go", "exec", "vet", "./src/plugins/nix/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("   [ERROR] Go vet failed: %v\n", err)
	} else {
		fmt.Println("   [PASS] Go vet")
	}

	cmd = exec.Command("./dialtone.sh", "go", "exec", "fmt", "./src/plugins/nix/...")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		fmt.Printf("   [WARN] Go fmt modified files:\n%s", out)
	} else {
		fmt.Println("   [PASS] Go fmt")
	}

	// 2. Lint TypeScript Code in versioned UI directories
	entries, _ := os.ReadDir(nixDir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "src_v") {
			uiDir := filepath.Join(nixDir, entry.Name(), "ui")
			if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err == nil {
				fmt.Printf(">> [NIX] Lint: checking TypeScript in %s...\n", uiDir)

				// Use bun directly as requested
				cmd := exec.Command("bun", "run", "lint")
				cmd.Dir = uiDir
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					fmt.Printf("   [ERROR] TypeScript lint failed in %s: %v\n", entry.Name(), err)
				} else {
					fmt.Printf("   [PASS] TypeScript lint in %s\n", entry.Name())
				}
			}
		}
	}

	fmt.Println(">> [NIX] Lint: complete")
	return nil
}
