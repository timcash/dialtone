package main

import (
	"os"
	"path/filepath"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	smoke "dialtone/dev/plugins/task/src_v1/test/01_smoke"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)
	defer func() {
		if r := recover(); r != nil {
			logs.Error("[PROCESS][PANIC] task src_v1 test runner panic: %v", r)
			os.Exit(1)
		}
	}()

	repoRoot, err := findRepoRoot()
	if err != nil {
		logs.Error("task test init failed: %v", err)
		os.Exit(1)
	}

	reg := testv1.NewRegistry()
	smoke.Register(reg)
	logs.Info("Running task src_v1 tests in single process (%d steps)", len(reg.Steps))

	err = reg.Run(testv1.SuiteOptions{
		Version:       "task-io-linking-v1",
		ReportPath:    filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "test", "TEST.md"),
		LogPath:       filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "test", "test.log"),
		ErrorLogPath:  filepath.Join(repoRoot, "src", "plugins", "task", "src_v1", "test", "error.log"),
		NATSURL:       "nats://127.0.0.1:4222",
		NATSSubject:   "logs.test.task-io-linking-v1",
		AutoStartNATS: true,
	})
	if err != nil {
		logs.Error("[PROCESS][ERROR] task src_v1 tests failed: %v", err)
		logs.Error("task src_v1 tests failed: %v", err)
		os.Exit(1)
	}
	logs.Info("task src_v1 tests passed")
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(cwd, "dialtone.sh")); err == nil {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return "", logs.Errorf("repo root not found")
		}
		cwd = parent
	}
}
