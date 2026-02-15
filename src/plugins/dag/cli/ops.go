package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func RunInstall(versionDir string) error {
	fmt.Printf(">> [DAG] Install: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "install", "--force")
	return cmd.Run()
}

func RunFmt(versionDir string) error {
	fmt.Printf(">> [DAG] Fmt: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "fmt", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunFormat(versionDir string) error {
	fmt.Printf(">> [DAG] Format: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "format")
	return cmd.Run()
}

func RunVet(versionDir string) error {
	fmt.Printf(">> [DAG] Vet: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "vet", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunGoBuild(versionDir string) error {
	fmt.Printf(">> [DAG] Go Build: %s\n", versionDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "build", "./src/plugins/dag/"+versionDir+"/...")
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunServe(versionDir string) error {
	fmt.Printf(">> [DAG] Serve: %s\n", versionDir)
	cwd, _ := os.Getwd()
	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "cmd", "main.go")))
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunUIRun(versionDir string, extraArgs []string) error {
	fmt.Printf(">> [DAG] UI Run: %s\n", versionDir)
	port := 3000
	if len(extraArgs) >= 2 && extraArgs[0] == "--port" {
		if p, err := strconv.Atoi(extraArgs[1]); err == nil {
			port = p
		}
	}

	cwd, _ := os.Getwd()
	uiDir := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "ui")
	cmd := runBun(cwd, uiDir, "run", "dev", "--host", "127.0.0.1", "--port", strconv.Itoa(port))
	return cmd.Run()
}

func RunTest(versionDir string) error {
	fmt.Printf(">> [DAG] Test: %s\n", versionDir)
	cwd, _ := os.Getwd()
	testPkg := "./" + filepath.ToSlash(filepath.Join("src", "plugins", "dag", versionDir, "test"))
	testMain := filepath.Join(cwd, "src", "plugins", "dag", versionDir, "test", "main.go")
	if _, err := os.Stat(testMain); os.IsNotExist(err) {
		return fmt.Errorf("test runner not found: %s/main.go", testPkg)
	}

	cmd := exec.Command(filepath.Join(cwd, "dialtone.sh"), "go", "exec", "run", testPkg)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
