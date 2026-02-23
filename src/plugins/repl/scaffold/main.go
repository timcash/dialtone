package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	repl "dialtone/dev/plugins/repl/src_v1/go/repl"
)

func main() {
	logs.SetOutput(os.Stdout)
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old repl CLI order is deprecated. Use: ./dialtone.sh repl src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("Unsupported repl version: %s", version)
		os.Exit(1)
	}

	switch command {
	case "test":
		if err := runVersionedTest(version); err != nil {
			logs.Error("REPL test error: %v", err)
			os.Exit(1)
		}
	case "run":
		if err := repl.RunLocal(nil, rest); err != nil {
			logs.Error("repl run failed: %v", err)
			os.Exit(1)
		}
	case "serve":
		if err := repl.RunServe(rest); err != nil {
			logs.Error("repl serve failed: %v", err)
			os.Exit(1)
		}
	case "join":
		if err := repl.RunJoin(rest); err != nil {
			logs.Error("repl join failed: %v", err)
			os.Exit(1)
		}
	case "status":
		if err := repl.RunStatus(rest); err != nil {
			logs.Error("repl status failed: %v", err)
			os.Exit(1)
		}
	case "service":
		if err := repl.RunService(rest); err != nil {
			logs.Error("repl service failed: %v", err)
			os.Exit(1)
		}
	case "build":
		if err := buildStandaloneBinary("", "", ""); err != nil {
			logs.Error("repl build failed: %v", err)
			os.Exit(1)
		}
	case "release":
		if err := runRelease(rest); err != nil {
			logs.Error("repl release failed: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("Unknown repl command: %s", command)
		printUsage()
		os.Exit(1)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return "", "", nil, false, fmt.Errorf("missing arguments")
	}
	if isHelp(args[0]) {
		return "src_v1", "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh repl src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first repl argument (usage: ./dialtone.sh repl src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func runVersionedTest(versionDir string) error {
	paths, err := repl.ResolvePaths("")
	if err != nil {
		return err
	}
	if versionDir != "src_v1" {
		return fmt.Errorf("unsupported repl version for tests: %s", versionDir)
	}
	goArgs := []string{"src_v1", "exec", "run", paths.TestCmdMain}
	fullArgs := append([]string{"go"}, goArgs...)
	cmd := exec.Command(filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh"), fullArgs...)
	cmd.Dir = paths.Runtime.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func buildStandaloneBinary(version, goos, goarch string) error {
	paths, err := repl.ResolvePaths("")
	if err != nil {
		return err
	}
	binDir := paths.StandaloneBinDir
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	name := "repl-src_v1"
	if goos != "" && goarch != "" {
		name = fmt.Sprintf("repl-src_v1-%s-%s", goos, goarch)
		if goos == "windows" {
			name += ".exe"
		}
	}
	out := filepath.Join(binDir, name)
	pkg := filepath.Join(paths.Preset.Cmd, "repld", "main.go")
	ld := ""
	if strings.TrimSpace(version) != "" {
		ld = "-X dialtone/dev/plugins/repl/src_v1/go/repl.BuildVersion=" + version
	}

	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		if fallback, lookErr := exec.LookPath("go"); lookErr == nil {
			goBin = fallback
		} else {
			return fmt.Errorf("managed go binary not found at %s and fallback go not in PATH", goBin)
		}
	}
	args := []string{"build"}
	if ld != "" {
		args = append(args, "-ldflags", ld)
	}
	args = append(args, "-o", out, pkg)

	cmd := exec.Command(goBin, args...)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	if goos != "" {
		cmd.Env = append(cmd.Env, "GOOS="+goos)
	}
	if goarch != "" {
		cmd.Env = append(cmd.Env, "GOARCH="+goarch)
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	logs.Raw("Built standalone REPL binary: %s", out)
	return nil
}

func runRelease(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing release subcommand (build|publish)")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "build":
		return releaseBuild(rest)
	case "publish":
		return releasePublish(rest)
	default:
		return fmt.Errorf("unknown release subcommand: %s", sub)
	}
}

func releaseBuild(args []string) error {
	version := ""
	if len(args) > 0 {
		version = strings.TrimSpace(args[0])
	}
	if version == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v1 release build <version>")
	}
	targets := [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}, {"windows", "amd64"}}
	for _, t := range targets {
		if err := buildStandaloneBinary(version, t[0], t[1]); err != nil {
			return err
		}
	}
	logs.Info("Built release binaries for %s", version)
	return nil
}

func releasePublish(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v1 release publish <version> [repo]")
	}
	version := strings.TrimSpace(args[0])
	repo := "timcash/dialtone"
	if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
		repo = strings.TrimSpace(args[1])
	}
	paths, err := repl.ResolvePaths("")
	if err != nil {
		return err
	}
	binDir := paths.StandaloneBinDir
	assets := []string{
		"repl-src_v1-linux-amd64",
		"repl-src_v1-linux-arm64",
		"repl-src_v1-darwin-amd64",
		"repl-src_v1-darwin-arm64",
		"repl-src_v1-windows-amd64.exe",
	}
	for _, a := range assets {
		if _, err := os.Stat(filepath.Join(binDir, a)); err != nil {
			return fmt.Errorf("missing asset %s (run release build first)", a)
		}
	}

	githubArgs := []string{"github", "src_v1", "release", "upsert", "--tag", version, "--repo", repo, "--title", "REPL " + version, "--notes", "Automated REPL release " + version}
	for _, a := range assets {
		githubArgs = append(githubArgs, "--asset", filepath.Join(binDir, a))
	}
	cmd := exec.Command(filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh"), githubArgs...)
	cmd.Dir = paths.Runtime.RepoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	logs.Info("Published release %s to %s", version, repo)
	return nil
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh repl src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  run [--name HOST]                                    Run local REPL session")
	logs.Raw("  serve [--nats-url URL] [--room NAME] [--embedded-nats] [--tsnet] [--hostname HOST]")
	logs.Raw("                                                       Start shared REPL host/service")
	logs.Raw("  join [--nats-url URL] [--room NAME] [--name HOST]   Join a shared REPL host")
	logs.Raw("  status [--nats-url URL] [--room NAME]               Show NATS/tsnet/chrome status")
	logs.Raw("  service [--mode install|run|status] [--repo owner/repo] [--nats-url URL] [--room NAME] [--check-interval 3m]")
	logs.Raw("                                                       install: register persistent OS service (default)")
	logs.Raw("                                                       run: start supervisor in foreground")
	logs.Raw("                                                       status: print OS service status")
	logs.Raw("  build                                                Build standalone repl-src_v1 binary")
	logs.Raw("  release build <version>                              Build per-architecture release binaries")
	logs.Raw("  release publish <version> [owner/repo]              Publish binaries to GitHub release")
	logs.Raw("  test                                                 Run REPL src_v1 tests")
	logs.Raw("  help                                                 Show this help")
}
