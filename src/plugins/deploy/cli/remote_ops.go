package cli

import (
	"dialtone/cli/src/core/logger"
	"dialtone/cli/src/core/ssh"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
)

func RunSyncCode(args []string) {
	fs := flag.NewFlagSet("sync-code", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		logger.LogFatal("Error: -host (user@host) and -pass are required for sync-code")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		home, err := ssh.GetRemoteHome(client)
		if err != nil {
			logger.LogFatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_src")
	}

	logger.LogInfo("Syncing code to %s on %s...", remoteDir, *host)

	// Clean remote src dir to remove stale files
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("rm -rf %s/src && mkdir -p %s/src", remoteDir, remoteDir))

	// Sync root files
	filesToUpload := []string{"go.mod", "go.sum", "dialtone.sh", "dialtone.ps1", "dialtone.cmd", "README.md"}
	for _, file := range filesToUpload {
		if _, err := os.Stat(file); err == nil {
			logger.LogInfo("Uploading %s...", file)
			if err := ssh.UploadFile(client, file, path.Join(remoteDir, file)); err != nil {
				logger.LogFatal("Failed to upload %s: %v", file, err)
			}
		}
	}

	// Sync src/*.go
	srcFiles, _ := filepath.Glob("src/*.go")
	for _, f := range srcFiles {
		logger.LogInfo("Uploading %s...", f)
		// Maintain src/ prefix for remote path
		if err := ssh.UploadFile(client, f, path.Join(remoteDir, filepath.ToSlash(f))); err != nil {
			logger.LogFatal("Failed to upload %s: %v", f, err)
		}
	}

	// Sync src/core
	if _, err := os.Stat("src/core"); err == nil {
		logger.LogInfo("Uploading src/core...")
		ssh.UploadDirFiltered(client, "src/core", path.Join(remoteDir, "src/core"), []string{".git"})
	}

	// Sync drone UI source (src/core/web now)
	if _, err := os.Stat("src/core/web"); err == nil {
		logger.LogInfo("Uploading src/core/web source...")
		ssh.UploadDirFiltered(client, "src/core/web", path.Join(remoteDir, "src/core/web"), []string{".git", "node_modules", "dist"})
	}

	// Sync src/plugins
	if _, err := os.Stat("src/plugins"); err == nil {
		logger.LogInfo("Uploading src/plugins...")
		ssh.UploadDirFiltered(client, "src/plugins", path.Join(remoteDir, "src/plugins"), []string{".git", "node_modules", "dist"})
	}

	// Sync test directory
	if _, err := os.Stat("test"); err == nil {
		logger.LogInfo("Uploading test directory...")
		ssh.UploadDirFiltered(client, "test", path.Join(remoteDir, "test"), []string{".git"})
	}

	// Sync mavlink directory (if it exists outside src - unlikely now?)
	if _, err := os.Stat("mavlink"); err == nil {
		logger.LogInfo("Uploading mavlink directory...")
		ssh.UploadDirFiltered(client, "mavlink", path.Join(remoteDir, "mavlink"), []string{".git", "__pycache__"})
	}

	logger.LogInfo("Code sync complete.")
}

func RunRemoteBuild(args []string) {
	fs := flag.NewFlagSet("remote-build", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		logger.LogFatal("Error: -host (user@host) and -pass are required for remote-build")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		logger.LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		home, err := ssh.GetRemoteHome(client)
		if err != nil {
			logger.LogFatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_src")
	}

	logger.LogInfo("Building on remote %s...", *host)

	// Build Drone UI (src/core/web)
	webCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s/src/core/web
		echo "Installing npm dependencies..."
		npm install
		echo "Building web assets..."
		npm run build
	`, remoteDir)

	output, err := ssh.RunSSHCommand(client, webCmd)
	if err != nil {
		logger.LogFatal("Remote web build failed: %v\nOutput: %s", err, output)
	}
	logger.LogInfo("%s", output)

	// Build Go
	goCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s
		echo "Building Go binary..."
		go build -v -o dialtone .
	`, remoteDir)

	output, err = ssh.RunSSHCommand(client, goCmd)
	if err != nil {
		logger.LogFatal("Remote Go build failed: %v\nOutput: %s", err, output)
	}
	logger.LogInfo("%s", output)
	logger.LogInfo("Remote build successful.")
}
