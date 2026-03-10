package repl

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunTest(args []string) error {
	fs := flag.NewFlagSet("repl-v3-test", flag.ContinueOnError)
	real := fs.Bool("real", false, "Run real integration steps (cloudflare/tsnet)")
	requireEmbeddedTSNet := fs.Bool("require-embedded-tsnet", false, "Fail if native tailscale is active during tsnet step")
	wslHost := fs.String("wsl-host", "", "Override WSL host used by test steps")
	wslUser := fs.String("wsl-user", "", "Override WSL user used by test steps")
	tunnelName := fs.String("tunnel-name", "", "Override cloudflare tunnel name used by test steps")
	tunnelURL := fs.String("tunnel-url", "", "Override cloudflare tunnel URL used by test steps")
	installURL := fs.String("install-url", "", "Override bootstrap install.sh URL for tmp bootstrap mode")
	bootstrapRepoURL := fs.String("bootstrap-repo-url", "", "Override bootstrap repo tarball URL for tmp bootstrap mode")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *real {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_REAL", "1")
	}
	if *requireEmbeddedTSNet {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_REQUIRE_EMBEDDED_TSNET", "1")
	}
	if strings.TrimSpace(*wslHost) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_WSL_HOST", strings.TrimSpace(*wslHost))
	}
	if strings.TrimSpace(*wslUser) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_WSL_USER", strings.TrimSpace(*wslUser))
	}
	if strings.TrimSpace(*tunnelName) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_TUNNEL_NAME", strings.TrimSpace(*tunnelName))
	}
	if strings.TrimSpace(*tunnelURL) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_TUNNEL_URL", strings.TrimSpace(*tunnelURL))
	}
	if strings.TrimSpace(*installURL) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_INSTALL_URL", strings.TrimSpace(*installURL))
	}
	if strings.TrimSpace(*bootstrapRepoURL) != "" {
		_ = os.Setenv("DIALTONE_REPL_V3_TEST_BOOTSTRAP_REPO_URL", strings.TrimSpace(*bootstrapRepoURL))
	}

	rest := fs.Args()
	if strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_MODE")) == "inside" {
		return runInRepoTest(rest)
	}
	return runTmpBootstrapTest(rest)
}

func RunTestClean(args []string) error {
	fs := flag.NewFlagSet("repl-v3-test-clean", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "List temp folders without deleting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	tmpRoot := strings.TrimSpace(os.TempDir())
	if tmpRoot == "" {
		return fmt.Errorf("temp directory is empty")
	}
	entries, err := os.ReadDir(tmpRoot)
	if err != nil {
		return err
	}
	const prefix = "dialtone-repl-v3-bootstrap-"
	matches := make([]string, 0, 16)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		matches = append(matches, filepath.Join(tmpRoot, name))
	}
	if len(matches) == 0 {
		logs.Info("test-clean: no %s* folders found in %s", prefix, tmpRoot)
		return nil
	}
	if *dryRun {
		for _, p := range matches {
			logs.Info("test-clean dry-run: %s", p)
		}
		logs.Info("test-clean dry-run complete: %d folder(s) matched", len(matches))
		return nil
	}
	removed := 0
	for _, p := range matches {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
		removed++
		logs.Info("test-clean removed: %s", p)
	}
	logs.Info("test-clean complete: %d folder(s) removed", removed)
	return nil
}

func runInRepoTest(args []string) error {
	repoRoot, srcRoot, err := resolveRoots()
	if err != nil {
		return err
	}
	goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN"))
	if goBin == "" {
		goBin = "go"
	}
	cmdArgs := []string{"run", "./plugins/repl/src_v3/test/cmd/main.go"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "DIALTONE_REPO_ROOT="+repoRoot, "DIALTONE_SRC_ROOT="+srcRoot)
	return cmd.Run()
}

func runTmpBootstrapTest(args []string) error {
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return err
	}
	tmpRoot, err := os.MkdirTemp("", "dialtone-repl-v3-bootstrap-*")
	if err != nil {
		return err
	}
	tmpRepo := filepath.Join(tmpRoot, "repo")
	tmpEnv := filepath.Join(tmpRoot, "dialtone_env")
	if err := os.MkdirAll(tmpRepo, 0o755); err != nil {
		return err
	}
	if entries, err := os.ReadDir(tmpRepo); err != nil {
		return err
	} else if len(entries) != 0 {
		return fmt.Errorf("expected empty tmp repo at start, found %d entries in %s", len(entries), tmpRepo)
	}
	tmpConfig := filepath.Join(tmpRepo, "env", "dialtone.json")
	_ = os.Remove(tmpConfig)

	installURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_INSTALL_URL"))
	serverURL := ""
	serverPort := 0
	if installURL == "" {
		repoTar := filepath.Join(tmpRoot, "dialtone-local.tar.gz")
		if err := createRepoTarball(repoRoot, repoTar); err != nil {
			return err
		}
		srcDialtone := filepath.Join(repoRoot, "dialtone.sh")
		localURL, localPort, closeServer, err := startLocalBootstrapServer(repoTar, srcDialtone)
		if err != nil {
			return err
		}
		defer closeServer()
		serverURL = localURL
		serverPort = localPort
		installURL = fmt.Sprintf("http://shell.dialtone.earth:%d/install.sh", serverPort)
	}
	bootstrapRepoURL := strings.TrimSpace(os.Getenv("DIALTONE_REPL_V3_TEST_BOOTSTRAP_REPO_URL"))
	if bootstrapRepoURL == "" && serverURL != "" {
		bootstrapRepoURL = serverURL + "/dialtone-main.tar.gz"
	}

	logs.Info("REPL v3 bootstrap test temp root: %s", tmpRoot)
	logs.Info("REPL v3 bootstrap test starts with no config file at: %s", tmpConfig)
	logs.Info("REPL v3 bootstrap install URL: %s", installURL)
	if serverURL != "" {
		logs.Info("REPL v3 bootstrap test local server URL: %s", serverURL)
	} else {
		logs.Info("REPL v3 bootstrap test mode: external install URL (no local bootstrap server)")
	}
	logs.Info("REPL v3 bootstrap test command: (cd %s && curl -fsSL %s | bash -s -- --test)", tmpRepo, installURL)
	logs.Info("REPL v3 bootstrap inject demo command:")
	logs.Info("  ./dialtone.sh repl src_v3 inject --user llm-codex repl src_v3 bootstrap --apply --wsl-host wsl.shad-artichoke.ts.net --wsl-user user")

	curlCmd := fmt.Sprintf("curl -fsSL %s | bash -s -- --test", installURL)
	if serverPort > 0 {
		curlCmd = fmt.Sprintf("curl -fsSL --resolve shell.dialtone.earth:%d:127.0.0.1 %s | bash -s -- --test", serverPort, installURL)
	}
	bootstrap := exec.Command("bash", "-lc", curlCmd)
	bootstrap.Dir = tmpRepo
	bootstrap.Stdout = os.Stdout
	bootstrap.Stderr = os.Stderr
	bootstrap.Stdin = os.Stdin
	env := append(os.Environ(),
		"TEST_ANS_ENV="+tmpEnv,
		"TEST_ANS_REPO="+tmpRepo,
		"DIALTONE_USE_NIX=0",
		"DIALTONE_REPL_V3_TEST_MODE=inside",
		"DIALTONE_REPL_NATS_URL=nats://127.0.0.1:47222",
	)
	if bootstrapRepoURL != "" {
		env = append(env, "DIALTONE_BOOTSTRAP_REPO_URL="+bootstrapRepoURL)
	}
	bootstrap.Env = env
	bootstrap.Args = append(bootstrap.Args, args...)
	return bootstrap.Run()
}
