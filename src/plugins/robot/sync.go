package robot

import (
	"dialtone/cli/src/core/logger"
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func RunSyncCode(versionDir string, args []string) {
	fs := flag.NewFlagSet("sync-code", flag.ExitOnError)
	host := fs.String("host", os.Getenv("ROBOT_HOST"), "SSH host")
	user := fs.String("user", os.Getenv("ROBOT_USER"), "SSH user")
	fs.Parse(args)

	if *host == "" {
		logger.LogFatal("Error: --host or ROBOT_HOST is required")
	}
	if *user == "" {
		*user = os.Getenv("USER")
	}

	remoteHome := os.Getenv("REMOTE_DIR_SRC")
	if remoteHome == "" {
		remoteHome = fmt.Sprintf("/home/%s/dialtone_src", *user)
	}

	logger.LogInfo("[SYNC] Syncing code to %s:%s...", *host, remoteHome)

	cwd, _ := os.Getwd()

	// Ensure remote directory exists
	mkdirCmd := exec.Command("ssh", fmt.Sprintf("%s@%s", *user, *host), "mkdir", "-p", remoteHome)
	if err := mkdirCmd.Run(); err != nil {
		logger.LogFatal("Failed to create remote directory: %v", err)
	}

	// Use rsync to sync the whole project root but exclude heavy/unnecessary items.
	// We use "." as source to preserve the full directory structure on the destination.
	rsyncArgs := []string{
		"-avz",
		"--delete",
		// Exclude version control and tool-specific metadata
		"--exclude", ".git/",
		"--exclude", ".dialtone/",
		"--exclude", ".chrome_data/",
		"--exclude", ".vercel/",
		"--exclude", ".claude/",
		// Exclude build artifacts and binary outputs
		"--exclude", "bin/",
		"--exclude", "dist/",
		"--exclude", "out/",
		"--exclude", "*/ui/dist/",
		"--exclude", ".pixi/",
		// Exclude ALL node_modules everywhere in the tree
		"--exclude", "node_modules/",
		"--exclude", "**/node_modules/",
		// Exclude heavy dependency directories
		"--exclude", "dialtone_dependencies/",
		"--exclude", "dialtone_dev_dependencies/",
		// Exclude local environment files
		"--exclude", "env/.env",
		"--exclude", "*.log",
		// Source and Destination
		".",
		fmt.Sprintf("%s@%s:%s", *user, *host, remoteHome),
	}

	logger.LogInfo("Running rsync...")
	cmd := exec.Command("rsync", rsyncArgs...)
	cmd.Dir = cwd
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.LogFatal("rsync failed: %v", err)
	}

	logger.LogInfo("[SYNC] Code sync complete.")
	logger.LogInfo("[SYNC] Now run the following on the robot:")
	logger.LogInfo("  cd %s", remoteHome)
	logger.LogInfo("  ./dialtone.sh robot install %s", versionDir)
	logger.LogInfo("  ./dialtone.sh robot build %s", versionDir)
}
