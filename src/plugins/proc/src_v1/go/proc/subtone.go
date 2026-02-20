package proc

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RunSubtone(args []string) {
	cwd, _ := os.Getwd()
	repoRoot := cwd
	if filepath.Base(cwd) == "src" {
		repoRoot = filepath.Dir(cwd)
	}
	dialtoneSh := filepath.Join(repoRoot, "dialtone.sh")

	cmd := exec.Command(dialtoneSh, args...)
	cmd.Dir = repoRoot

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		fmt.Printf("DIALTONE> Failed to start subtone: %v\n", err)
		return
	}

	pid := cmd.Process.Pid
	TrackProcess(pid, args)
	defer UntrackProcess(pid)

	fmt.Printf("\nDIALTONE> Spawning subtone subprocess via PID %d...\n", pid)
	fmt.Printf("DIALTONE> Streaming stdout/stderr from subtone PID %d.\n", pid)

	// Combine stdout and stderr
	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// If the line already starts with DIALTONE>, strip it to avoid double prefixing
		// when streaming subtone output that was produced by another dev.go instance.
		displayLine := line
		if strings.HasPrefix(line, "DIALTONE> ") {
			displayLine = line[len("DIALTONE> "):]
		}
		prefix := fmt.Sprintf("DIALTONE:%d> ", pid)
		fmt.Printf("%s%s\n", prefix, displayLine)
	}

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	fmt.Printf("DIALTONE> Process %d exited with code %d.\n", pid, exitCode)
}
