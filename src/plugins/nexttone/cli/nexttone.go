package cli

import (
	"fmt"
	"os"
	"strings"
)

func Run(args []string) {
	if len(args) == 0 {
		RunNext(nil)
		return
	}

	switch args[0] {
	case "help", "--help", "-h":
		printUsage()
		return
	case "next":
		RunNext(args[1:])
	case "list":
		RunList()
	case "add":
		RunAdd(args[1:])
	case "subtone":
		RunSubtone(args[1:])
	default:
		// Treat unknown args as nexttone --sign
		if strings.HasPrefix(args[0], "--sign") {
			RunNext(args)
			return
		}
		fmt.Printf("Unknown nexttone subcommand: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Nexttone (beta) - microtone workflow driver")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  ./dialtone.sh nexttone                   # show current microtone prompt")
	fmt.Println("  ./dialtone.sh nexttone next              # alias of default behavior")
	fmt.Println("  ./dialtone.sh nexttone list              # show microtone graph + subtone list")
	fmt.Println("  ./dialtone.sh nexttone add <tone-name>   # add a tone and scaffold test")
	fmt.Println("  ./dialtone.sh nexttone subtone add <name> [--desc \"...\"]")
	fmt.Println("  ./dialtone.sh nexttone subtone set <name> --<field> \"...\"")
	fmt.Println("  ./dialtone.sh nexttone --sign yes|no     # record signature and advance")
	fmt.Println("  ./dialtone.sh nexttone help              # show this help")
	fmt.Println("")
	fmt.Println("Signing:")
	fmt.Println("  Use --sign yes|no to acknowledge the current prompt.")
	fmt.Println("  If --sign is missing or invalid, the same prompt repeats.")
	fmt.Println("  Tone names must be 3 to 5 kebab-case words.")
	fmt.Println("")
	fmt.Println("Environment:")
	fmt.Println("  NEXTTONE_DB_PATH  Override nexttone DB path (default: src/nexttone/<tone>/<tone>.duckdb)")
	fmt.Println("  NEXTTONE_TONE     Active tone name (default: default)")
	fmt.Println("  NEXTTONE_TONE_DIR Tone root directory (default: src/nexttone)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  ./dialtone.sh nexttone")
	fmt.Println("  ./dialtone.sh nexttone --sign yes")
	fmt.Println("  ./dialtone.sh nexttone list")
	fmt.Println("  ./dialtone.sh nexttone add www-nexttone-section")
	fmt.Println("  ./dialtone.sh nexttone subtone add nexttone-graph --desc \"add graph viz\"")
	fmt.Println("  ./dialtone.sh nexttone subtone set nexttone-graph --test-command \"./dialtone.sh plugin test nexttone\"")
}
