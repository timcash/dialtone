package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RunLint() error {
	fmt.Println(">> [WSL] Lint: starting...")
	cwd, _ := os.Getwd()
	wslDir := filepath.Join(cwd, "src", "plugins", "wsl")

	// 1. Lint Go Code
	fmt.Println(">> [WSL] Lint: checking Go code...")
	cmd := exec.Command("go", "vet", "./src/plugins/wsl/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("   [ERROR] Go vet failed: %v\n", err)
	} else {
		fmt.Println("   [PASS] Go vet")
	}

	cmd = exec.Command("go", "fmt", "./src/plugins/wsl/...")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		fmt.Printf("   [WARN] Go fmt modified files:\n%s", out)
	} else {
		fmt.Println("   [PASS] Go fmt")
	}

	// 2. Lint TypeScript Code in versioned UI directories
	entries, _ := os.ReadDir(wslDir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "src_v") {
			uiDir := filepath.Join(wslDir, entry.Name(), "ui")
			if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err == nil {
				fmt.Printf(">> [WSL] Lint: checking TypeScript in %s...\n", uiDir)

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

	fmt.Println(">> [WSL] Lint: complete")
	return nil
}
