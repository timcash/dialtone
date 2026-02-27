package main

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	version, command, rest, warnedOldOrder, err := parseArgs(os.Args[1:])
	if err != nil {
		logs.Error("%v", err)
		printUsage()
		os.Exit(1)
	}
	if warnedOldOrder {
		logs.Warn("old camera CLI order is deprecated. Use: ./dialtone.sh camera src_v1 <command> [args]")
	}
	if version != "src_v1" {
		logs.Error("unsupported camera version: %s", version)
		os.Exit(1)
	}

	switch command {
	case "build":
		if err := runBuild(rest); err != nil {
			logs.Error("camera build failed: %v", err)
			os.Exit(1)
		}
	case "run":
		if err := runCameraCommand("run", rest); err != nil {
			logs.Error("camera run failed: %v", err)
			os.Exit(1)
		}
	case "test":
		if err := runCameraTests(); err != nil {
			logs.Error("camera test failed: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		printUsage()
	default:
		logs.Error("unknown camera command: %s", command)
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
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh camera src_v1 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return "", "", nil, false, fmt.Errorf("expected version as first camera argument (usage: ./dialtone.sh camera src_v1 <command> [args])")
}

func isHelp(s string) bool {
	return s == "help" || s == "-h" || s == "--help"
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh camera src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  build   Build dialtone_camera_v1 binary (supports podman cross-build with cache)")
	logs.Raw("  run     Run camera runtime command")
	logs.Raw("  test    Run camera go tests")
	logs.Raw("  help    Show this help")
	logs.Raw("")
	logs.Raw("Build examples:")
	logs.Raw("  ./dialtone.sh camera src_v1 build")
	logs.Raw("  ./dialtone.sh camera src_v1 build --goos linux --goarch arm64 --podman")
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("camera-build", flag.ContinueOnError)
	goos := fs.String("goos", "linux", "Target GOOS")
	goarch := fs.String("goarch", "arm64", "Target GOARCH")
	out := fs.String("out", "", "Output binary path (default: <repo>/bin/dialtone_camera_v1-<goos>-<goarch>)")
	podman := fs.Bool("podman", true, "Use podman cross-build path when target differs from host")
	if err := fs.Parse(args); err != nil {
		return err
	}

	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	repoRoot := rt.RepoRoot
	srcRoot := rt.SrcRoot

	output := strings.TrimSpace(*out)
	if output == "" {
		output = filepath.Join(repoRoot, "bin", fmt.Sprintf("dialtone_camera_v1-%s-%s", *goos, *goarch))
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}

	hostMatches := *goos == runtime.GOOS && *goarch == runtime.GOARCH
	if hostMatches || !*podman {
		return buildLocal(srcRoot, output, *goos, *goarch)
	}
	return buildWithPodman(repoRoot, srcRoot, output, *goos, *goarch)
}

func buildLocal(srcRoot, output, goos, goarch string) error {
	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return fmt.Errorf("go binary not found (managed and PATH)")
		}
		goBin = fallback
	}
	cmd := exec.Command(goBin, "build", "-o", output, "./plugins/camera/src_v1/cmd/main.go")
	cmd.Dir = srcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1", "GOOS="+goos, "GOARCH="+goarch)
	logs.Info("camera build local: %s/%s -> %s", goos, goarch, output)
	return cmd.Run()
}

func buildWithPodman(repoRoot, srcRoot, output, goos, goarch string) error {
	if _, err := exec.LookPath("podman"); err != nil {
		return fmt.Errorf("podman is required for cross-compilation but not found in PATH")
	}

	imageName := "dialtone-builder-arm"
	dockerfilePath := filepath.Join(repoRoot, "containers", "Dockerfile.arm")
	logs.Info("camera build podman image: %s", imageName)
	buildImg := exec.Command("podman", "build", "-t", imageName, "-f", dockerfilePath, ".")
	buildImg.Dir = repoRoot
	buildImg.Stdout = os.Stdout
	buildImg.Stderr = os.Stderr
	if err := buildImg.Run(); err != nil {
		return fmt.Errorf("podman build failed: %w", err)
	}

	absOut, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	absRepo, err := filepath.Abs(repoRoot)
	if err != nil {
		return err
	}
	relOut, err := filepath.Rel(absRepo, absOut)
	if err != nil {
		return err
	}
	if strings.HasPrefix(relOut, "..") {
		return fmt.Errorf("camera build --out must be inside repo root for podman build: %s", output)
	}
	remoteOut := "/repo/" + filepath.ToSlash(relOut)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}

	goModCache := filepath.Join(os.Getenv("HOME"), "go", "pkg", "mod")
	if _, err := os.Stat(goModCache); os.IsNotExist(err) {
		altCache := filepath.Join(logs.GetDialtoneEnv(), "go", "pkg", "mod")
		if _, altErr := os.Stat(altCache); altErr == nil {
			goModCache = altCache
		}
	}

	podmanArgs := []string{
		"run", "--rm",
		"-v", repoRoot + ":/repo:z",
		"-w", "/repo/src",
		"-e", "CGO_ENABLED=1",
		"-e", "GOOS=" + goos,
		"-e", "GOARCH=" + goarch,
		"-e", "GOPATH=/go",
	}
	if cc := crossCompilerFor(goarch); cc != "" {
		podmanArgs = append(podmanArgs, "-e", "CC="+cc)
	}
	if _, err := os.Stat(goModCache); err == nil {
		podmanArgs = append(podmanArgs, "-v", goModCache+":/go/pkg/mod:z")
	}
	podmanArgs = append(podmanArgs, imageName, "go", "build", "-o", remoteOut, "./plugins/camera/src_v1/cmd/main.go")

	logs.Info("camera build podman: %s/%s -> %s", goos, goarch, output)
	runCmd := exec.Command("podman", podmanArgs...)
	runCmd.Dir = repoRoot
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("podman run build failed: %w", err)
	}

	if _, err := os.Stat(output); err != nil {
		return fmt.Errorf("expected podman-built binary missing: %s", output)
	}
	if err := os.Chmod(output, 0o755); err != nil {
		return err
	}
	return nil
}

func crossCompilerFor(goarch string) string {
	switch strings.TrimSpace(goarch) {
	case "arm64", "aarch64":
		return "aarch64-linux-gnu-gcc"
	case "arm", "armv7":
		return "arm-linux-gnueabihf-gcc"
	case "amd64", "x86_64":
		return "x86_64-linux-gnu-gcc"
	default:
		return ""
	}
}

func runCameraCommand(command string, args []string) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return fmt.Errorf("go binary not found (managed and PATH)")
		}
		goBin = fallback
	}
	cmdArgs := []string{"run", "./plugins/camera/src_v1/cmd/main.go", command}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(goBin, cmdArgs...)
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runCameraTests() error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	goBin := filepath.Join(logs.GetDialtoneEnv(), "go", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		fallback, lookErr := exec.LookPath("go")
		if lookErr != nil {
			return fmt.Errorf("go binary not found (managed and PATH)")
		}
		goBin = fallback
	}
	cmd := exec.Command(goBin, "test", "./plugins/camera/...")
	cmd.Dir = rt.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
