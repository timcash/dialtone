package swarmv3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	sshplugin "dialtone/dev/plugins/ssh/src_v1/go"
	sshlib "golang.org/x/crypto/ssh"
)

const defaultVersion = "src_v3"

type registerResponse struct {
	Peers []struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	} `json:"peers"`
}

func Run(args []string) error {
	version, command, rest, warnedOldOrder, err := parseArgs(args)
	if err != nil {
		printUsage()
		return err
	}
	if warnedOldOrder {
		logs.Warn("old swarm CLI order is deprecated. Use: ./dialtone.sh swarm src_v3 <command> [args]")
	}
	if version == "" {
		version = defaultVersion
	}
	if version != defaultVersion {
		return fmt.Errorf("unsupported version %s (expected %s)", version, defaultVersion)
	}

	paths, err := ResolvePaths("")
	if err != nil {
		return err
	}
	_ = configv1.LoadEnvFile(paths.Runtime)
	_ = configv1.ApplyRuntimeEnv(paths.Runtime)

	switch command {
	case "help", "-h", "--help":
		printUsage()
		return nil
	case "install":
		return runInstall(paths, rest)
	case "build":
		return runBuild(paths, rest)
	case "test":
		return runTest(paths, rest)
	case "deploy":
		return runDeploy(paths, rest)
	case "verify-host-builds":
		return runVerifyHostBuilds(paths, rest)
	case "relay":
		return runRelay(paths, rest)
	default:
		printUsage()
		return fmt.Errorf("unknown swarm command: %s", command)
	}
}

func parseArgs(args []string) (version, command string, rest []string, warnedOldOrder bool, err error) {
	if len(args) == 0 {
		return defaultVersion, "help", nil, false, nil
	}
	if isHelp(args[0]) {
		return defaultVersion, "help", nil, false, nil
	}
	if strings.HasPrefix(args[0], "src_v") {
		if len(args) < 2 {
			return "", "", nil, false, fmt.Errorf("missing command (usage: ./dialtone.sh swarm src_v3 <command> [args])")
		}
		return args[0], args[1], args[2:], false, nil
	}
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return args[1], args[0], args[2:], true, nil
	}
	return defaultVersion, args[0], args[1:], false, nil
}

func isHelp(s string) bool {
	switch strings.TrimSpace(s) {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func printUsage() {
	logs.Raw("Usage: ./dialtone.sh swarm src_v3 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  install                       Install build/runtime dependencies")
	logs.Raw("  build [--arch host|x86_64|arm64|all]")
	logs.Raw("                                Build static binaries")
	logs.Raw("  test [--mode local|rendezvous|all] [--rendezvous-url URL]")
	logs.Raw("                                Run local and/or rendezvous tests")
	logs.Raw("  deploy --host H --user U --pass P [--port 22] [--remote-path PATH]")
	logs.Raw("                                Build for remote arch and upload via SSH")
	logs.Raw("  verify-host-builds [--hosts chroma,darkmac,legion] [--repo-dir ~/dialtone]")
	logs.Raw("                     [--host H --user U --pass P --port 22 --name custom]")
	logs.Raw("                                SSH each host and run native host build")
	logs.Raw("  relay serve [--listen :8080]  Run local rendezvous web server")
	logs.Raw("  help                          Show this help")
}

func runInstall(paths Paths, args []string) error {
	fs := flag.NewFlagSet("swarm-install", flag.ContinueOnError)
	withARM64 := fs.Bool("with-arm64", true, "Install ARM64 cross-compiler dependencies")
	skipApt := fs.Bool("skip-apt", false, "Skip apt install commands")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*skipApt {
		pkgs := []string{
			"curl", "git", "build-essential", "cmake", "ninja-build",
			"clang", "lld", "libuv1-dev", "libuv1", "pkg-config", "python3",
			"nodejs", "npm",
		}
		if *withARM64 {
			pkgs = append(pkgs,
				"gcc-aarch64-linux-gnu",
				"g++-aarch64-linux-gnu",
				"binutils-aarch64-linux-gnu",
			)
		}
		if err := runCmd("", "sudo", "apt-get", "update"); err != nil {
			return err
		}
		installArgs := append([]string{"apt-get", "install", "-y"}, pkgs...)
		if err := runCmd("", "sudo", installArgs...); err != nil {
			return err
		}
	}

	if _, err := exec.LookPath("bare-make"); err != nil {
		if err := runCmd("", "sudo", "npm", "install", "-g", "bare-runtime", "bare-make"); err != nil {
			return err
		}
	}

	if err := ensureLibudx(paths); err != nil {
		return err
	}
	if err := runCmd(paths.LibudxDir, "npm", "install"); err != nil {
		return err
	}
	logs.Info("swarm src_v3 install complete")
	return nil
}

func runBuild(paths Paths, args []string) error {
	fs := flag.NewFlagSet("swarm-build", flag.ContinueOnError)
	arch := fs.String("arch", "host", "host|x86_64|arm64|all")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := ensureLibudx(paths); err != nil {
		return err
	}
	if err := buildLibudxNative(paths); err != nil {
		return err
	}

	targets := expandArch(*arch)
	for _, a := range targets {
		switch a {
		case "x86_64":
			if err := buildAMD64(paths); err != nil {
				return err
			}
		case "arm64":
			if err := buildARM64(paths); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported arch %s", a)
		}
	}
	return nil
}

func runTest(paths Paths, args []string) error {
	fs := flag.NewFlagSet("swarm-test", flag.ContinueOnError)
	mode := fs.String("mode", "all", "local|rendezvous|all")
	rendezvousURL := fs.String("rendezvous-url", "https://relay.dialtone.earth", "Rendezvous URL")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cmd := exec.Command(goBin(paths.Runtime), "run", "./plugins/swarm/src_v3/test/cmd/main.go",
		"--mode", strings.TrimSpace(*mode),
		"--rendezvous-url", strings.TrimSpace(*rendezvousURL),
	)
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runDeploy(paths Paths, args []string) error {
	fs := flag.NewFlagSet("swarm-deploy", flag.ContinueOnError)
	host := fs.String("host", strings.TrimSpace(os.Getenv("ROBOT_HOST")), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", strings.TrimSpace(os.Getenv("ROBOT_USER")), "SSH user")
	pass := fs.String("pass", strings.TrimSpace(os.Getenv("ROBOT_PASSWORD")), "SSH password")
	remotePath := fs.String("remote-path", "", "Remote binary path")
	aliasPath := fs.String("alias-path", "", "Optional remote symlink path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*host) == "" || strings.TrimSpace(*user) == "" || strings.TrimSpace(*pass) == "" {
		return errors.New("deploy requires --host, --user, and --pass (or ROBOT_* env vars)")
	}

	client, err := sshplugin.DialSSH(strings.TrimSpace(*host), strings.TrimSpace(*port), strings.TrimSpace(*user), strings.TrimSpace(*pass))
	if err != nil {
		return err
	}
	defer client.Close()

	archOut, err := sshplugin.RunSSHCommand(client, "uname -m")
	if err != nil {
		return err
	}
	targetArch := normalizeRemoteArch(archOut)
	if targetArch == "" {
		return fmt.Errorf("unsupported remote arch output: %s", strings.TrimSpace(archOut))
	}
	if err := runBuild(paths, []string{"--arch", targetArch}); err != nil {
		return err
	}

	localBin := paths.BinAMD64
	if targetArch == "arm64" {
		localBin = paths.BinARM64
	}
	rp := strings.TrimSpace(*remotePath)
	if rp == "" {
		rp = fmt.Sprintf("/home/%s/dialtone_swarm_v3_%s", strings.TrimSpace(*user), targetArch)
	}
	if _, err := sshplugin.RunSSHCommand(client, "mkdir -p "+shellQuote(filepath.Dir(rp))); err != nil {
		return err
	}

	tmpRemote := rp + ".upload-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := sshplugin.UploadFile(client, localBin, tmpRemote); err != nil {
		return err
	}
	if _, err := sshplugin.RunSSHCommand(client, "chmod +x "+shellQuote(tmpRemote)+" && mv -f "+shellQuote(tmpRemote)+" "+shellQuote(rp)); err != nil {
		return err
	}

	ap := strings.TrimSpace(*aliasPath)
	if ap != "" {
		if _, err := sshplugin.RunSSHCommand(client, "ln -sfn "+shellQuote(rp)+" "+shellQuote(ap)); err != nil {
			return err
		}
	}
	logs.Info("deployed %s -> %s (%s)", localBin, rp, targetArch)
	return nil
}

type buildHostSpec struct {
	Name string
	Host string
	Port string
	User string
	Pass string
}

func runVerifyHostBuilds(paths Paths, args []string) error {
	fs := flag.NewFlagSet("swarm-verify-host-builds", flag.ContinueOnError)
	hostsFlag := fs.String("hosts", firstNonEmpty(os.Getenv("SWARM_BUILD_HOSTS"), "chroma,darkmac,legion"), "Comma-separated host names: robot,chroma,darkmac,legion")
	host := fs.String("host", "", "Single SSH host override")
	port := fs.String("port", "22", "Single SSH port override")
	user := fs.String("user", "", "Single SSH user override")
	pass := fs.String("pass", "", "Single SSH password override")
	name := fs.String("name", "custom", "Single host display name")
	repoDir := fs.String("repo-dir", "~/dialtone", "Remote repo directory")
	install := fs.Bool("install", false, "Run install before build on remote hosts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	specs := []buildHostSpec{}
	if strings.TrimSpace(*host) != "" {
		if strings.TrimSpace(*user) == "" || strings.TrimSpace(*pass) == "" {
			return fmt.Errorf("single-host mode requires --user and --pass")
		}
		specs = append(specs, buildHostSpec{
			Name: strings.TrimSpace(*name),
			Host: strings.TrimSpace(*host),
			Port: strings.TrimSpace(*port),
			User: strings.TrimSpace(*user),
			Pass: strings.TrimSpace(*pass),
		})
	} else {
		var err error
		specs, err = resolveBuildHostSpecs(*hostsFlag)
		if err != nil {
			return err
		}
	}
	remoteRepo := strings.TrimSpace(*repoDir)
	if remoteRepo == "" {
		remoteRepo = "~/dialtone"
	}
	var failed []string
	for _, spec := range specs {
		logs.Info("verify host build: %s (%s@%s:%s)", spec.Name, spec.User, spec.Host, spec.Port)
		if err := verifySingleHostBuild(spec, remoteRepo, *install); err != nil {
			logs.Error("host %s failed: %v", spec.Name, err)
			failed = append(failed, spec.Name)
			continue
		}
		logs.Info("host %s build verification passed", spec.Name)
	}
	if len(failed) > 0 {
		return fmt.Errorf("verify-host-builds failed for: %s", strings.Join(failed, ", "))
	}
	return nil
}

func verifySingleHostBuild(spec buildHostSpec, repoDir string, install bool) error {
	client, err := sshplugin.DialSSH(spec.Host, spec.Port, spec.User, spec.Pass)
	if err != nil {
		return err
	}
	defer client.Close()

	osName, archName, err := detectRemoteTarget(client)
	if err != nil {
		return err
	}
	logs.Info("remote target %s: %s/%s", spec.Name, osName, archName)

	installCmd := ""
	if install {
		switch osName {
		case "linux":
			installCmd = "./dialtone.sh swarm src_v3 install"
		case "darwin":
			installCmd = "./dialtone.sh swarm src_v3 install --skip-apt"
		default:
			return fmt.Errorf("unsupported remote OS %q", osName)
		}
	}
	cmdParts := []string{
		"cd " + shellQuote(repoDir),
	}
	if installCmd != "" {
		cmdParts = append(cmdParts, installCmd)
	}
	cmdParts = append(cmdParts,
		"./dialtone.sh swarm src_v3 build --arch host",
		"ls -lh src/plugins/swarm/src_v3/dialtone_swarm_v3_*",
	)
	out, err := sshplugin.RunSSHCommand(client, strings.Join(cmdParts, " && "))
	if err != nil {
		return err
	}
	logs.Info("remote build output (%s):\n%s", spec.Name, strings.TrimSpace(out))
	return nil
}

func runRelay(paths Paths, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		logs.Raw("Usage: ./dialtone.sh swarm src_v3 relay serve [--listen :8080]")
		return nil
	}
	if args[0] != "serve" {
		return fmt.Errorf("unknown relay subcommand %s", args[0])
	}
	fs := flag.NewFlagSet("swarm-relay-serve", flag.ContinueOnError)
	listen := fs.String("listen", ":8080", "listen address")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	cmd := exec.Command(goBin(paths.Runtime), "run", "./plugins/swarm/src_v3/relay_web/main.go")
	cmd.Dir = paths.Runtime.SrcRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"RELAY_LISTEN="+strings.TrimSpace(*listen),
		"RELAY_STATIC_DIR="+filepath.Join(paths.VersionDir, "relay_web", "static"),
	)
	return cmd.Run()
}

func ensureLibudx(paths Paths) error {
	if _, err := os.Stat(paths.LibudxDir); err == nil {
		return nil
	}
	return runCmd(paths.Runtime.RepoRoot, "git", "submodule", "update", "--init", "--recursive", "src/plugins/swarm/src_v3/libudx")
}

func buildLibudxNative(paths Paths) error {
	if err := runCmd(paths.LibudxDir, "npm", "install"); err != nil {
		return err
	}
	if err := runCmd(paths.LibudxDir, "bare-make", "generate"); err != nil {
		return err
	}
	return runCmd(paths.LibudxDir, "bare-make", "build")
}

func buildAMD64(paths Paths) error {
	udxLib, err := findFile(paths.LibudxDir, "build", "libudx.a")
	if err != nil {
		return err
	}
	uvLib, err := findFile(paths.LibudxDir, "build", "libuv.a")
	if err != nil {
		return err
	}
	args := []string{
		paths.SourceFile,
		"-O2", "-Wall", "-Wextra",
		"-I" + filepath.Join(paths.LibudxDir, "include"),
		udxLib, uvLib,
		"-static", "-lpthread", "-ldl", "-lrt", "-lm",
		"-o", paths.BinAMD64,
	}
	return runCmd(paths.VersionDir, "gcc", args...)
}

func buildARM64(paths Paths) error {
	if _, err := exec.LookPath("aarch64-linux-gnu-gcc"); err != nil {
		return fmt.Errorf("missing aarch64-linux-gnu-gcc (run install first)")
	}
	buildDir := filepath.Join(paths.LibudxDir, "build-arm64-local")
	if err := runCmd(paths.VersionDir, "cmake",
		"-S", paths.LibudxDir,
		"-B", buildDir,
		"-G", "Ninja",
		"-DCMAKE_SYSTEM_NAME=Linux",
		"-DCMAKE_SYSTEM_PROCESSOR=aarch64",
		"-DCMAKE_C_COMPILER=aarch64-linux-gnu-gcc",
		"-DCMAKE_CXX_COMPILER=aarch64-linux-gnu-g++",
	); err != nil {
		return err
	}
	if err := runCmd(paths.VersionDir, "cmake", "--build", buildDir, "-j"); err != nil {
		return err
	}
	udxLib, err := findFile(paths.LibudxDir, "build-arm64-local", "libudx.a")
	if err != nil {
		return err
	}
	uvLib, err := findFile(paths.LibudxDir, "build-arm64-local", "libuv.a")
	if err != nil {
		return err
	}
	args := []string{
		paths.SourceFile,
		"-O2", "-Wall", "-Wextra",
		"-I" + filepath.Join(paths.LibudxDir, "include"),
		udxLib, uvLib,
		"-static", "-lpthread", "-ldl", "-lrt", "-lm",
		"-o", paths.BinARM64,
	}
	return runCmd(paths.VersionDir, "aarch64-linux-gnu-gcc", args...)
}

func RunLocalSelfTest(bin string) error {
	helpOut, err := runCapture(bin, []string{"--help"}, 5*time.Second)
	if err != nil {
		return err
	}
	if !strings.Contains(helpOut, "Usage:") {
		return fmt.Errorf("help output missing Usage")
	}

	if out, err := runCapture(bin, []string{"--bind-ip", "127.0.0.1"}, 4*time.Second); err == nil || !strings.Contains(out, "--bind-port is required") {
		return fmt.Errorf("expected missing --bind-port validation")
	}

	tmp, err := os.MkdirTemp("", "swarm-v3-local-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	receiverLog := filepath.Join(tmp, "receiver.log")
	senderLog := filepath.Join(tmp, "sender.log")
	receiverFile, _ := os.Create(receiverLog)
	defer receiverFile.Close()

	ctxReceiver, cancelReceiver := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelReceiver()
	receiver := exec.CommandContext(ctxReceiver, bin,
		"--bind-ip", "127.0.0.1", "--bind-port", "19002",
		"--peer-ip", "127.0.0.1", "--peer-port", "19001",
		"--local-id", "2", "--peer-id", "1",
		"--no-send", "--exit-after-ms", "2200",
	)
	receiver.Stdout = receiverFile
	receiver.Stderr = receiverFile
	if err := receiver.Start(); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)

	if _, err := runCaptureToFile(bin, []string{
		"--bind-ip", "127.0.0.1", "--bind-port", "19001",
		"--peer-ip", "127.0.0.1", "--peer-port", "19002",
		"--local-id", "1", "--peer-id", "2",
		"--message", "test-payload", "--count", "2", "--interval-ms", "200",
		"--exit-after-ms", "1200",
	}, senderLog, 4*time.Second); err != nil {
		return err
	}
	_ = receiver.Wait()

	recvData, _ := os.ReadFile(receiverLog)
	if !strings.Contains(string(recvData), "received[") || !strings.Contains(string(recvData), "test-payload") {
		return fmt.Errorf("local receiver did not capture payload")
	}
	logs.Info("local test passed")
	return nil
}

func RunRendezvousSelfTest(bin, rendezvousURL string) error {
	if rendezvousURL == "" {
		return fmt.Errorf("rendezvous URL is required")
	}
	hc := &http.Client{Timeout: 5 * time.Second}
	if err := checkHealth(hc, rendezvousURL); err != nil {
		return err
	}
	topic := fmt.Sprintf("swarm-v3-relay-%d", time.Now().UnixNano())
	aPort := 19401
	bPort := 19402
	msg := "relay-discovery-ok"

	if _, err := registerPeer(hc, rendezvousURL, topic, "node-a", aPort); err != nil {
		return err
	}
	if _, err := registerPeer(hc, rendezvousURL, topic, "node-b", bPort); err != nil {
		return err
	}
	a2, err := registerPeer(hc, rendezvousURL, topic, "node-a", aPort)
	if err != nil {
		return err
	}
	b1, err := registerPeer(hc, rendezvousURL, topic, "node-b", bPort)
	if err != nil {
		return err
	}
	if len(a2.Peers) == 0 || len(b1.Peers) == 0 {
		return fmt.Errorf("rendezvous did not return peers")
	}
	aPeerIP := a2.Peers[0].IP
	aPeerPort := a2.Peers[0].Port
	bPeerIP := b1.Peers[0].IP
	bPeerPort := b1.Peers[0].Port
	logs.Info("rendezvous discovered: node-a->%s:%d node-b->%s:%d", aPeerIP, aPeerPort, bPeerIP, bPeerPort)
	bindA := "127.0.0.1"
	bindB := "127.0.0.1"
	aPeerIP, bPeerIP = "127.0.0.1", "127.0.0.1"

	tmp, err := os.MkdirTemp("", "swarm-v3-rendezvous-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	receiverLog := filepath.Join(tmp, "receiver.log")
	senderLog := filepath.Join(tmp, "sender.log")
	receiverFile, _ := os.Create(receiverLog)
	defer receiverFile.Close()

	ctxReceiver, cancelReceiver := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancelReceiver()
	receiver := exec.CommandContext(ctxReceiver, bin,
		"--bind-ip", bindA, "--bind-port", strconv.Itoa(aPort),
		"--peer-ip", aPeerIP, "--peer-port", strconv.Itoa(aPeerPort),
		"--local-id", "2", "--peer-id", "1",
		"--no-send", "--exit-after-ms", "2600",
	)
	receiver.Stdout = receiverFile
	receiver.Stderr = receiverFile
	if err := receiver.Start(); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)

	if _, err := runCaptureToFile(bin, []string{
		"--bind-ip", bindB, "--bind-port", strconv.Itoa(bPort),
		"--peer-ip", bPeerIP, "--peer-port", strconv.Itoa(bPeerPort),
		"--local-id", "1", "--peer-id", "2",
		"--message", msg, "--count", "2", "--interval-ms", "200",
		"--exit-after-ms", "1200",
	}, senderLog, 4*time.Second); err != nil {
		return err
	}
	_ = receiver.Wait()

	recvData, _ := os.ReadFile(receiverLog)
	if !strings.Contains(string(recvData), "received[") || !strings.Contains(string(recvData), msg) {
		return fmt.Errorf("rendezvous test receiver did not capture payload")
	}
	logs.Info("rendezvous test passed (%s)", rendezvousURL)
	return nil
}

func checkHealth(client *http.Client, rendezvousURL string) error {
	resp, err := client.Get(strings.TrimRight(rendezvousURL, "/") + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("rendezvous health failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func registerPeer(client *http.Client, rendezvousURL, topic, who string, port int) (registerResponse, error) {
	payload := map[string]any{"topic": topic, "who": who, "port": port}
	b, _ := json.Marshal(payload)
	resp, err := client.Post(strings.TrimRight(rendezvousURL, "/")+"/api/register", "application/json", bytes.NewReader(b))
	if err != nil {
		return registerResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return registerResponse{}, fmt.Errorf("register failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return registerResponse{}, err
	}
	return out, nil
}

func runCmd(dir, bin string, args ...string) error {
	logs.Info("run: %s %s", bin, strings.Join(args, " "))
	cmd := exec.Command(bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runCapture(bin string, args []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runCaptureToFile(bin string, args []string, file string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	_ = os.WriteFile(file, out, 0o644)
	return string(out), err
}

func findFile(root, containsDir, base string) (string, error) {
	targetDir := filepath.Join(root, containsDir)
	var found string
	_ = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == base {
			found = path
			return io.EOF
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("file %s not found under %s", base, targetDir)
	}
	return found, nil
}

func binaryForHost(paths Paths) string {
	if hostArch() == "arm64" {
		return paths.BinARM64
	}
	return paths.BinAMD64
}

func EnsureHostBinary(paths Paths) (string, error) {
	host := hostArch()
	if host == "arm64" {
		if _, err := os.Stat(paths.BinARM64); err != nil {
			if err := runBuild(paths, []string{"--arch", "arm64"}); err != nil {
				return "", err
			}
		}
		return paths.BinARM64, nil
	}
	if _, err := os.Stat(paths.BinAMD64); err != nil {
		if err := runBuild(paths, []string{"--arch", "x86_64"}); err != nil {
			return "", err
		}
	}
	return paths.BinAMD64, nil
}

func hostArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	default:
		return "x86_64"
	}
}

func expandArch(arch string) []string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "all":
		return []string{"x86_64", "arm64"}
	case "arm64", "aarch64":
		return []string{"arm64"}
	case "x86_64", "amd64":
		return []string{"x86_64"}
	case "host", "":
		return []string{hostArch()}
	default:
		return []string{arch}
	}
}

func normalizeRemoteArch(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "x86_64", "amd64":
		return "x86_64"
	case "aarch64", "arm64":
		return "arm64"
	default:
		return ""
	}
}

func detectRemoteTarget(client *sshlib.Client) (string, string, error) {
	osOut, err := sshplugin.RunSSHCommand(client, "uname -s")
	if err != nil {
		return "", "", fmt.Errorf("detect remote os failed: %w", err)
	}
	archOut, err := sshplugin.RunSSHCommand(client, "uname -m")
	if err != nil {
		return "", "", fmt.Errorf("detect remote arch failed: %w", err)
	}
	osName := strings.ToLower(strings.TrimSpace(osOut))
	archName := strings.ToLower(strings.TrimSpace(archOut))

	goos := "linux"
	switch osName {
	case "linux":
		goos = "linux"
	case "darwin":
		goos = "darwin"
	default:
		return "", "", fmt.Errorf("unsupported remote OS %q", osName)
	}

	goarch := "arm64"
	switch archName {
	case "aarch64", "arm64":
		goarch = "arm64"
	case "x86_64", "amd64":
		goarch = "amd64"
	default:
		return "", "", fmt.Errorf("unsupported remote arch %q", archName)
	}
	return goos, goarch, nil
}

func resolveBuildHostSpecs(raw string) ([]buildHostSpec, error) {
	names := strings.Split(strings.TrimSpace(raw), ",")
	out := make([]buildHostSpec, 0, len(names))
	for _, name := range names {
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "" {
			continue
		}
		switch n {
		case "robot":
			spec := buildHostSpec{
				Name: "robot",
				Host: strings.TrimSpace(os.Getenv("ROBOT_HOST")),
				Port: firstNonEmpty(os.Getenv("ROBOT_PORT"), "22"),
				User: strings.TrimSpace(os.Getenv("ROBOT_USER")),
				Pass: strings.TrimSpace(os.Getenv("ROBOT_PASSWORD")),
			}
			if spec.Host == "" || spec.User == "" || spec.Pass == "" {
				return nil, fmt.Errorf("robot host credentials missing (ROBOT_HOST/ROBOT_USER/ROBOT_PASSWORD)")
			}
			out = append(out, spec)
		case "chroma":
			spec := buildHostSpec{
				Name: "chroma",
				Host: firstNonEmpty(os.Getenv("CHROMA_HOST"), "chroma"),
				Port: firstNonEmpty(os.Getenv("CHROMA_PORT"), "22"),
				User: firstNonEmpty(os.Getenv("CHROMA_USER"), "dev"),
				Pass: strings.TrimSpace(os.Getenv("CHROMA_PASSWORD")),
			}
			if spec.Pass == "" {
				return nil, fmt.Errorf("chroma password missing (CHROMA_PASSWORD)")
			}
			out = append(out, spec)
		case "darkmac":
			spec := buildHostSpec{
				Name: "darkmac",
				Host: firstNonEmpty(os.Getenv("DARKMAC_HOST"), "darkmac"),
				Port: firstNonEmpty(os.Getenv("DARKMAC_PORT"), "22"),
				User: firstNonEmpty(os.Getenv("DARKMAC_USER"), "tim"),
				Pass: strings.TrimSpace(os.Getenv("DARKMAC_PASSWORD")),
			}
			if spec.Pass == "" {
				return nil, fmt.Errorf("darkmac password missing (DARKMAC_PASSWORD)")
			}
			out = append(out, spec)
		case "legion":
			spec := buildHostSpec{
				Name: "legion",
				Host: firstNonEmpty(os.Getenv("LEGION_HOST"), "legion"),
				Port: firstNonEmpty(os.Getenv("LEGION_PORT"), "22"),
				User: firstNonEmpty(os.Getenv("LEGION_USER"), "tim"),
				Pass: strings.TrimSpace(os.Getenv("LEGION_PASSWORD")),
			}
			if spec.Pass == "" {
				return nil, fmt.Errorf("legion password missing (LEGION_PASSWORD)")
			}
			out = append(out, spec)
		default:
			return nil, fmt.Errorf("unsupported host name %q", n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts configured")
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func goBin(rt configv1.Runtime) string {
	if strings.TrimSpace(rt.GoBin) != "" {
		return rt.GoBin
	}
	return "go"
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
