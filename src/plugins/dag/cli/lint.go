package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RunLint(versionDir string) error {
	fmt.Println(">> [DAG] Lint: starting...")
	cwd, _ := os.Getwd()
	dagDir := filepath.Join(cwd, "src", "plugins", "dag")

	// 1. Lint Go Code (use dialtone go toolchain)
	fmt.Println(">> [DAG] Lint: checking Go code...")
	cmd := exec.Command("./dialtone.sh", "go", "exec", "vet", "./src/plugins/dag/...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go vet failed: %v", err)
	}
	fmt.Println("   [PASS] Go vet")

	cmd = exec.Command("./dialtone.sh", "go", "exec", "fmt", "./src/plugins/dag/...")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		fmt.Printf("   [WARN] Go fmt modified files:\n%s", out)
	} else {
		fmt.Println("   [PASS] Go fmt")
	}

	// 2. Lint TypeScript Code in versioned UI directories
	entries, _ := os.ReadDir(dagDir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "src_v") {
			if versionDir != "" && entry.Name() != versionDir {
				continue
			}
			uiDir := filepath.Join(dagDir, entry.Name(), "ui")
			if _, err := os.Stat(filepath.Join(uiDir, "package.json")); err == nil {
				fmt.Printf(">> [DAG] Lint: checking TypeScript in %s...\n", uiDir)

				installCmd := exec.Command("bun", "install")
				installCmd.Dir = uiDir
				installCmd.Stdout = os.Stdout
				installCmd.Stderr = os.Stderr
				if err := installCmd.Run(); err != nil {
					return fmt.Errorf("bun install failed in %s: %v", entry.Name(), err)
				}

				cmd := exec.Command("bun", "run", "lint")
				cmd.Dir = uiDir
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr

				if err := cmd.Run(); err != nil {
					return fmt.Errorf("typescript lint failed in %s: %v", entry.Name(), err)
				}
				fmt.Printf("   [PASS] TypeScript lint in %s\n", entry.Name())
			}
		}
	}

	fmt.Println(">> [DAG] Lint: complete")
	return nil
}
