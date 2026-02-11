package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	msg := "GO_STDERR_DEFAULT"
	code := 17

	if len(os.Args) > 1 {
		msg = os.Args[1]
	}
	if len(os.Args) > 2 {
		if parsed, err := strconv.Atoi(os.Args[2]); err == nil {
			code = parsed
		}
	}

	fmt.Fprintln(os.Stderr, msg)
	os.Exit(code)
}
