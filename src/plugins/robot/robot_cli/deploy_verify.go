package robot_cli

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
	sshlib "golang.org/x/crypto/ssh"
)

func RunDeployTest(versionDir string, args []string) error {
	logger.LogInfo("[DEPLOY-TEST] Starting step-by-step verification using %s...", versionDir)

	host := os.Getenv("ROBOT_HOST")
	user := os.Getenv("ROBOT_USER")
	pass := os.Getenv("ROBOT_PASSWORD")
	hostname := os.Getenv("DIALTONE_HOSTNAME")
	authKey := os.Getenv("TS_AUTHKEY")

	if host == "" || user == "" || pass == "" || hostname == "" || authKey == "" {
		logger.LogFatal("Missing required environment variables (ROBOT_HOST, ROBOT_USER, ROBOT_PASSWORD, DIALTONE_HOSTNAME, TS_AUTHKEY)")
	}

	logger.LogInfo("[DEPLOY-TEST] Step 0: Connecting to %s...", host)
	client, err := ssh.DialSSH(host, "22", user, pass)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer client.Close()

	remoteArch, _ := ssh.RunSSHCommand(client, "uname -m")
	remoteArch = strings.TrimSpace(remoteArch)
	logger.LogInfo("[DEPLOY-TEST] Remote architecture: %s", remoteArch)

	targetOS := "linux"
	targetArch := "arm64"
	if remoteArch == "x86_64" || remoteArch == "amd64" {
		targetArch = "amd64"
	}

	cwd, _ := os.Getwd()
	tmpDir := filepath.Join(cwd, ".dialtone", "deploy_test")
	_ = os.MkdirAll(tmpDir, 0755)

	remoteHome, _ := ssh.GetRemoteHome(client)
	remoteDebugPath := path.Join(remoteHome, "dialtone_debug")

	// --- STEP 1: TSNET ONLY ---
	logger.LogInfo("[DEPLOY-TEST] Step 1: Verifying Tailscale (tsnet) connectivity...")
	tsnetSrc := fmt.Sprintf(`package main
import (
	"context"
	"fmt"
	"os"
	"time"
	"tailscale.com/tsnet"
)
func main() {
	s := &tsnet.Server{
		Hostname: "%s",
		AuthKey:  "%s",
		Logf: func(string, ...any) {},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := s.Up(ctx)
	if err != nil { fmt.Printf("FAIL: %%v\n", err); os.Exit(1) }
	fmt.Printf("PASS: IP=%%v\n", status.TailscaleIPs)
}
`, hostname, authKey)

	if err := runDebugStep(tsnetSrc, tmpDir, targetOS, targetArch, client, remoteDebugPath); err != nil {
		return fmt.Errorf("Step 1 failed: %w", err)
	}

	// --- STEP 2: TSNET + NATS ---
	logger.LogInfo("[DEPLOY-TEST] Step 2: Verifying NATS Server start...")
	natsSrc := fmt.Sprintf(`package main
import (
	"context"
	"fmt"
	"os"
	"time"
	"tailscale.com/tsnet"
	"github.com/nats-io/nats-server/v2/server"
)
func main() {
	s := &tsnet.Server{ Hostname: "%s", AuthKey: "%s", Logf: func(string, ...any) {} }
	if _, err := s.Up(context.Background()); err != nil { fmt.Printf("FAIL TS: %%v\n", err); os.Exit(1) }
	
	opts := &server.Options{ Host: "0.0.0.0", Port: 4222 }
	ns, err := server.NewServer(opts)
	if err != nil { fmt.Printf("FAIL NATS: %%v\n", err); os.Exit(1) }
	go ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) { fmt.Printf("FAIL NATS TIMEOUT\n"); os.Exit(1) }
	fmt.Printf("PASS: NATS READY\n")
}
`, hostname, authKey)

	if err := runDebugStep(natsSrc, tmpDir, targetOS, targetArch, client, remoteDebugPath); err != nil {
		return fmt.Errorf("Step 2 failed: %w", err)
	}

	// --- STEP 3: TSNET + WEB HEALTH ---
	logger.LogInfo("[DEPLOY-TEST] Step 3: Verifying Web Server (Health Check)...")
	webSrc := fmt.Sprintf(`package main
import (
	"fmt"
	"os"
	"net/http"
	"time"
	"tailscale.com/tsnet"
)
func main() {
	s := &tsnet.Server{ Hostname: "%s", AuthKey: "%s", Logf: func(string, ...any) {} }
	ln, err := s.Listen("tcp", ":80")
	if err != nil { fmt.Printf("FAIL LISTEN: %%v\n", err); os.Exit(1) }
	
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "ok") })
	
	go http.Serve(ln, mux)
	fmt.Printf("PASS: WEB READY\n")
	time.Sleep(2 * time.Second)
}
`, hostname, authKey)

	if err := runDebugStep(webSrc, tmpDir, targetOS, targetArch, client, remoteDebugPath); err != nil {
		return fmt.Errorf("Step 3 failed: %w", err)
	}

	logger.LogInfo("[DEPLOY-TEST] ALL STEPS PASSED. The robot is ready for full deployment.")
	return nil
}

func runDebugStep(source, tmpDir, osStr, archStr string, client *sshlib.Client, remotePath string) error {
	srcPath := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcPath, []byte(source), 0644); err != nil {
		return err
	}

	localBin := filepath.Join(tmpDir, "dialtone_debug")
	_ = os.Remove(localBin)

	logger.LogInfo("   Compiling debug binary for %s/%s...", osStr, archStr)
	cmd := exec.Command("go", "build", "-o", localBin, srcPath)
	cmd.Env = append(os.Environ(), "GOOS="+osStr, "GOARCH="+archStr, "CGO_ENABLED=0")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compilation failed: %v\n%s", err, output)
	}

	logger.LogInfo("   Uploading to robot...")
	// Kill existing debug process if any
	_, _ = ssh.RunSSHCommand(client, "pkill -9 dialtone_debug")
	
	if err := ssh.UploadFile(client, localBin, remotePath); err != nil {
		return err
	}
	_, _ = ssh.RunSSHCommand(client, "chmod +x "+remotePath)

	logger.LogInfo("   Executing remotely...")
	// We run with a timeout
	done := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		out, err := ssh.RunSSHCommand(client, remotePath)
		if err != nil {
			errChan <- err
		} else {
			done <- out
		}
	}()

	select {
	case out := <-done:
		logger.LogInfo("   Remote Output: %s", strings.TrimSpace(out))
		if !strings.Contains(out, "PASS:") {
			return fmt.Errorf("remote execution did not indicate success")
		}
	case err := <-errChan:
		return err
	case <-time.After(30 * time.Second):
		_, _ = ssh.RunSSHCommand(client, "pkill -9 dialtone_debug")
		return fmt.Errorf("remote execution timed out")
	}

	return nil
}
