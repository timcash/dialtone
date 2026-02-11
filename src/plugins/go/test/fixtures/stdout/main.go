package main

import (
	"fmt"
	"os"
)

func main() {
	msg := "GO_STDOUT_DEFAULT"
	if len(os.Args) > 1 {
		msg = os.Args[1]
	}
	fmt.Println(msg)
}
