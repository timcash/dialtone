package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println("Hello from a Podman container!")
	fmt.Println("Running on OS:", runtime.GOOS)
	fmt.Printf("This is a minimal Go binary running in a 'scratch' container.\n")
}
