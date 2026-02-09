package dialtone

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runSwarmSrc(args []string) {
	n := ""
	for i, arg := range args {
		if arg == "--n" && i+1 < len(args) {
			n = args[i+1]
			break
		}
	}

	if n == "" {
		fmt.Println("Usage: ./dialtone.sh swarm src --n <number>")
		return
	}

	targetDir := fmt.Sprintf("src%s", n)
	targetPath := filepath.Join("src", "plugins", "swarm", targetDir)
	templatePath := filepath.Join("src", "plugins", "swarm", "src_v2")

	if _, err := os.Stat(targetPath); err == nil {
		fmt.Printf("[swarm] Folder %s already exists. Validating files...\n", targetDir)
		validateFiles(targetPath)
		return
	}

	fmt.Printf("[swarm] Creating %s from template...\n", targetDir)
	if err := copyDir(templatePath, targetPath); err != nil {
		fmt.Printf("[swarm] Error creating folder: %v\n", err)
		return
	}
	fmt.Printf("[swarm] Successfully created %s\n", targetDir)
}

func validateFiles(dir string) {
	required := []string{
		"index.js", 
		"package.json", 
		"bare/warm.js", 
		"bare/dashboard.js", 
		"bare/autolog.js", 
		"bare/autokv.js",
		"ui/index.html",
		"ui/package.json",
		"ui/vite.config.ts",
		"ui/src/main.ts",
		"ui/src/style.css",
	}
	for _, f := range required {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err != nil {
			fmt.Printf("[swarm] Missing file: %s\n", f)
		} else {
			fmt.Printf("[swarm] Valid: %s\n", f)
		}
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		return copyFile(path, targetPath)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}