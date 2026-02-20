package main

import (
	"fmt"
	"os"
)

func main() {
	got := Add(2, 2)
	if got != 4 {
		fmt.Printf("FAIL: Add(2,2) expected 4, got %d\n", got)
		os.Exit(1)
	}
	fmt.Println("PASS: Add(2,2)=4")
}
