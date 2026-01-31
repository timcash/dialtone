package main

import (
	"fmt"
	"path/filepath"

	nexttone_cli "dialtone/cli/src/plugins/nexttone/cli"
)

func main() {
	path := filepath.Join("src", "plugins", "nexttone", "init.duckdb")
	if err := nexttone_cli.InitDB(path); err != nil {
		fmt.Printf("initdb failed: %v\n", err)
		return
	}
	fmt.Printf("initdb created: %s\n", path)
}
