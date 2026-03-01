package main

import (
	"bytes"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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
	case "stream":
		if err := runStream(rest); err != nil {
			logs.Error("camera stream failed: %v", err)
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
	logs.Raw("  stream  Stream-test a remote camera host over ssh mesh and save one snapshot")
	logs.Raw("  test    Run camera go tests")
	logs.Raw("  help    Show this help")
	logs.Raw("")
	logs.Raw("Build examples:")
	logs.Raw("  ./dialtone.sh camera src_v1 build")
	logs.Raw("  ./dialtone.sh camera src_v1 build --goos linux --goarch arm64 --podman")
	logs.Raw("")
	logs.Raw("Stream test example:")
	logs.Raw("  ./dialtone.sh camera src_v1 stream --host rover --pass password1")
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

func runStream(args []string) error {
	fs := flag.NewFlagSet("camera-stream", flag.ContinueOnError)
	host := fs.String("host", "rover", "SSH mesh host alias (for example rover)")
	user := fs.String("user", "", "SSH user (optional)")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	port := fs.String("port", "", "SSH port (optional)")
	remotePort := fs.Int("remote-port", 19090, "Remote camera HTTP port")
	timeout := fs.Duration("timeout", 12*time.Second, "Timeout for health/frame fetch")
	snapshot := fs.String("snapshot", filepath.Join(os.TempDir(), "camera_stream_snapshot.jpg"), "Local snapshot output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	node, err := sshplugin.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	client, node, hostAddr, usePort, err := sshplugin.DialMeshNode(node.Name, sshplugin.CommandOptions{
		User:     strings.TrimSpace(*user),
		Port:     strings.TrimSpace(*port),
		Password: strings.TrimSpace(*pass),
	})
	if err != nil {
		return err
	}
	defer client.Close()
	logs.Info("camera stream dialing host=%s addr=%s port=%s", node.Name, hostAddr, usePort)

	localPort, err := allocateLocalPort()
	if err != nil {
		return err
	}
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	remoteAddr := fmt.Sprintf("127.0.0.1:%d", *remotePort)
	if err := sshplugin.ForwardRemoteToLocal(client, remoteAddr, localAddr); err != nil {
		return err
	}

	baseURL := fmt.Sprintf("http://%s", localAddr)
	httpClient := &http.Client{Timeout: *timeout}
	healthResp, err := httpClient.Get(baseURL + "/health")
	if err != nil {
		return fmt.Errorf("health request failed: %w", err)
	}
	_ = healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status=%d", healthResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, baseURL+"/stream", nil)
	if err != nil {
		return err
	}
	streamResp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stream request failed: %w", err)
	}
	defer streamResp.Body.Close()
	if streamResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(streamResp.Body, 256))
		return fmt.Errorf("stream status=%d body=%s", streamResp.StatusCode, strings.TrimSpace(string(body)))
	}
	frame, err := readFirstJPEG(streamResp.Body, *timeout)
	if err != nil {
		return err
	}
	if err := os.WriteFile(strings.TrimSpace(*snapshot), frame, 0o644); err != nil {
		return err
	}
	logs.Info("camera stream ok host=%s frame_bytes=%d snapshot=%s", node.Name, len(frame), *snapshot)
	return nil
}

func allocateLocalPort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("failed to allocate local tcp addr")
	}
	return addr.Port, nil
}

func readFirstJPEG(r io.Reader, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	var buf []byte
	tmp := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			if frame := extractJPEG(buf); len(frame) > 0 {
				return frame, nil
			}
			if len(buf) > 10*1024*1024 {
				return nil, fmt.Errorf("camera stream frame too large")
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return nil, fmt.Errorf("timed out waiting for JPEG frame")
}

func extractJPEG(data []byte) []byte {
	start := bytes.Index(data, []byte{0xFF, 0xD8})
	if start < 0 {
		return nil
	}
	endRel := bytes.Index(data[start:], []byte{0xFF, 0xD9})
	if endRel < 0 {
		return nil
	}
	end := start + endRel + 2
	out := make([]byte, end-start)
	copy(out, data[start:end])
	return out
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
