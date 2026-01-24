package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	app "dialtone/cli/src/plugins/camera/app"
)

func RunCamera(args []string) {
	if len(args) < 1 {
		printUsage()
		return
	}

	command := args[0]
	// subArgs := args[1:]

	switch command {
	case "snapshot":
		runSnapshot()
	case "stream":
		runStream()
	default:
		fmt.Printf("Unknown camera command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: dialtone-dev camera <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  snapshot   Capture a single frame to snapshot.jpg")
	fmt.Println("  stream     Start camera and log frame stats to stdout")
}

func runSnapshot() {
	fmt.Println("Taking snapshot...")
	ctx := context.Background()

	cameras, err := app.ListCameras()
	if err != nil {
		fmt.Printf("Error listing cameras: %v\n", err)
		return
	}
	if len(cameras) == 0 {
		fmt.Println("No cameras found")
		return
	}

	dev := cameras[0].Device
	fmt.Printf("Using device: %s\n", dev)

	if err := app.StartCamera(ctx, dev); err != nil {
		fmt.Printf("Error starting camera: %v\n", err)
		return
	}

	// Poll for a frame
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println("Timeout waiting for frame")
			return
		case <-ticker.C:
			frame, ts := app.GetLatestFrame()
			if frame != nil && len(frame) > 0 {
				fmt.Printf("Captured frame: %d bytes (Time: %s)\n", len(frame), ts.Format(time.RFC3339))
				if err := os.WriteFile("snapshot.jpg", frame, 0644); err != nil {
					fmt.Printf("Failed to save snapshot: %v\n", err)
				} else {
					fmt.Println("Saved to snapshot.jpg")
				}
				return
			}
		}
	}
}

func runStream() {
	fmt.Println("Starting stream test (Ctrl+C to stop)...")
	ctx := context.Background()

	cameras, err := app.ListCameras()
	if err != nil {
		fmt.Printf("Error listing cameras: %v\n", err)
		return
	}
	if len(cameras) == 0 {
		fmt.Println("No cameras found")
		return
	}

	dev := cameras[0].Device
	fmt.Printf("Using device: %s\n", dev)

	if err := app.StartCamera(ctx, dev); err != nil {
		fmt.Printf("Error starting camera: %v\n", err)
		return
	}

	// Loop and print stats
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastTs time.Time
	for {
		select {
		case <-ticker.C:
			frame, ts := app.GetLatestFrame()
			if frame != nil {
				age := time.Since(ts)
				fps := "0"
				if !ts.Equal(lastTs) {
					fps = ">0" // Simple liveness check
				}
				fmt.Printf("Status: Connected | Frame Size: %d bytes | Latency: %s | Liveness: %s\n", len(frame), age, fps)
				lastTs = ts
			} else {
				fmt.Println("Status: Waiting for frames...")
			}
		}
	}
}
