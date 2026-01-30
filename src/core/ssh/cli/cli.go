package cli

import (
	"dialtone/cli/src/core/ssh"
	"fmt"
)

// Run handles the 'ssh' command
func Run(args []string) {
	if len(args) > 0 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		printUsage()
		return
	}

	ssh.RunSSH(args)
}

func printUsage() {
	fmt.Println("Usage: dialtone ssh [options]")
	fmt.Println()
	fmt.Println("SSH tools for remote robot interaction (command execution, file transfer).")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --host        SSH host (user@host)")
	fmt.Println("  --port        SSH port (default: 22)")
	fmt.Println("  --user        SSH username (overrides user@host)")
	fmt.Println("  --pass        SSH password")
	fmt.Println("  --cmd         Command to execute remotely")
	fmt.Println("  --upload      Local file to upload")
	fmt.Println("  --dest        Remote destination path")
	fmt.Println("  --download    Remote file to download")
	fmt.Println("  --local-dest  Local destination for download")
	fmt.Println("  help          Show help for ssh command")
	fmt.Println("  --help        Show help for ssh command")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  dialtone ssh --cmd \"ls -l\"")
	fmt.Println("  dialtone ssh --upload ./binary --dest /home/user/binary")
}
