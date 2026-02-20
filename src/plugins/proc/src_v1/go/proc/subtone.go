package proc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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

	logDir := filepath.Join(repoRoot, ".dialtone", "logs")
	logger, err := NewSubtoneLogger(pid, args, logDir)
	if err != nil {
		fmt.Printf("DIALTONE> Failed to init logger: %v\n", err)
	} else {
		logger.StartHeartbeat(3 * time.Second)
		defer logger.Stop()
	}

	// Stream stdout (info) and stderr (errors) separately
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if logger != nil {
				logger.LogLine(line)
			}
		}
	}()

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if logger != nil {
			logger.LogError(line)
		}
	}

	err = cmd.Wait()
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
