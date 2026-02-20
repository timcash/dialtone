package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type TestOptions struct {
	RepoRoot           string
	PluginDir          string
	VersionDir         string
	Attach             bool
	CPS                int
	BaseURL            string
	DevBaseURL         string
	DevPort            int
	AttachURL          string
	TestPkg            string // relative to RepoRoot
	EnvPrefix          string // e.g. "DAG"
	EnsurePreviewFunc  func() error
}

func RunPluginTests(opts TestOptions) error {
	if opts.EnsurePreviewFunc != nil {
		if err := opts.EnsurePreviewFunc(); err != nil {
			return err
		}
	}

	cmd := exec.Command(filepath.Join(opts.RepoRoot, "dialtone.sh"), "go", "exec", "run", opts.TestPkg)
	cmd.Dir = opts.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	cmd.Env = append(
		os.Environ(),
		opts.EnvPrefix+"_TEST_ATTACH=0",
		opts.EnvPrefix+"_TEST_BASE_URL="+opts.BaseURL,
		opts.EnvPrefix+"_TEST_DEV_BASE_URL="+opts.DevBaseURL,
		opts.EnvPrefix+"_TEST_CPS="+strconv.Itoa(opts.CPS),
	)
	if opts.Attach {
		cmd.Env = append(cmd.Env, opts.EnvPrefix+"_TEST_ATTACH=1")
	}
	
	err := cmd.Run()
	
	// Post-test: Re-ensure preview if possible
	if opts.EnsurePreviewFunc != nil {
		_ = opts.EnsurePreviewFunc()
	}
	
	return err
}
