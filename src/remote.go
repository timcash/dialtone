package dialtone

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"dialtone/cli/src/core/ssh"
)


func RunSyncCode(args []string) {
	fs := flag.NewFlagSet("sync-code", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for sync-code")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		remoteDir = "/home/tim/dialtone_src"
	}

	LogInfo("Syncing code to %s on %s...", remoteDir, *host)

	// Clean remote src dir to remove stale files (like legacy manager.go)
	_, _ = ssh.RunSSHCommand(client, fmt.Sprintf("rm -rf %s/src && mkdir -p %s/src", remoteDir, remoteDir))

	// Sync root files
	filesToUpload := []string{"go.mod", "go.sum", "dialtone.go", "build.sh", "build.ps1", "README.md"}
	for _, file := range filesToUpload {
		if _, err := os.Stat(file); err == nil {
			LogInfo("Uploading %s...", file)
			if err := ssh.UploadFile(client, file, path.Join(remoteDir, file)); err != nil {
				LogFatal("Failed to upload %s: %v", file, err)
			}
		}
	}

	// Sync src/*.go
	srcFiles, _ := filepath.Glob("src/*.go")
	for _, f := range srcFiles {
		LogInfo("Uploading %s...", f)
		if err := ssh.UploadFile(client, f, path.Join(remoteDir, filepath.ToSlash(f))); err != nil {
			LogFatal("Failed to upload %s: %v", f, err)
		}
	}

	// Sync drone UI source (src/web)
	if _, err := os.Stat("src/web"); err == nil {
		LogInfo("Uploading src/web source...")
		ssh.UploadDirFiltered(client, "src/web", path.Join(remoteDir, "src/web"), []string{".git", "node_modules", "dist"})
	}

	// Sync test directory
	if _, err := os.Stat("test"); err == nil {
		LogInfo("Uploading test directory...")
		ssh.UploadDirFiltered(client, "test", path.Join(remoteDir, "test"), []string{".git"})
	}

	// Sync mavlink directory (if it exists)
	if _, err := os.Stat("mavlink"); err == nil {
		LogInfo("Uploading mavlink directory...")
		ssh.UploadDirFiltered(client, "mavlink", path.Join(remoteDir, "mavlink"), []string{".git", "__pycache__"})
	}

	LogInfo("Code sync complete.")
}

func RunRemoteBuild(args []string) {
	fs := flag.NewFlagSet("remote-build", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	port := fs.String("port", "22", "SSH port")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	pass := fs.String("pass", os.Getenv("ROBOT_PASSWORD"), "SSH password")
	fs.Parse(args)

	if *host == "" || *pass == "" {
		LogFatal("Error: -host (user@host) and -pass are required for remote-build")
	}

	client, err := ssh.DialSSH(*host, *port, *user, *pass)
	if err != nil {
		LogFatal("SSH connection failed: %v", err)
	}
	defer client.Close()

	remoteDir := os.Getenv("REMOTE_DIR_SRC")
	if remoteDir == "" {
		home, err := ssh.GetRemoteHome(client)
		if err != nil {
			LogFatal("Failed to get remote home: %v", err)
		}
		remoteDir = path.Join(home, "dialtone_src")
	}

	LogInfo("Building on remote %s...", *host)

	// Build Drone UI (src/web)
	webCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s/src/web
		echo "Installing npm dependencies..."
		npm install
		echo "Building web assets..."
		npm run build
	`, remoteDir)

	output, err := ssh.RunSSHCommand(client, webCmd)
	if err != nil {
		LogFatal("Remote web build failed: %v\nOutput: %s", err, output)
	}
	LogInfo(output)

	// Build Go
	goCmd := fmt.Sprintf(`
		export PATH=$PATH:/usr/local/go/bin
		cd %s
		echo "Building Go binary..."
		go build -v -o dialtone .
	`, remoteDir)

	output, err = ssh.RunSSHCommand(client, goCmd)
	if err != nil {
		LogFatal("Remote Go build failed: %v\nOutput: %s", err, output)
	}
	LogInfo(output)
	LogInfo("Remote build successful.")
}
