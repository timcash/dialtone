package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: proc <command>")
		return
	}

	cmd := os.Args[1]
	switch cmd {
	case "test":
		fmt.Println("Running proc test (sleeping for 10s)...")
		time.Sleep(10 * time.Second)
		fmt.Println("Proc test complete.")
	case "sleep":
		duration := 5 * time.Second
		if len(os.Args) > 2 {
			if d, err := strconv.Atoi(os.Args[2]); err == nil {
				duration = time.Duration(d) * time.Second
			}
		}
		fmt.Printf("Sleeping for %v...\n", duration)
		time.Sleep(duration)
		fmt.Println("Sleep complete.")
	default:
		fmt.Printf("Unknown proc command: %s\n", cmd)
	}
}
