package proc

import (
	"fmt"
	"sync"
	"time"
)

func RunTestSrcV1() {
	fmt.Println("\nDIALTONE> Starting 3 parallel task workers for testing...")

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 1; i <= 3; i++ {
		go func(id int) {
			defer wg.Done()
			// Stagger start slightly
			time.Sleep(time.Duration(id*100) * time.Millisecond)
			args := []string{"proc", "sleep", "2"}
			_ = RunTaskWorker(args)
		}(i)
	}

	// We don't wait here because we want to return control to REPL so user can run 'ps'.
	// But if we return, the main loop prints USER-1>.
	// The task workers will run in background.
}
