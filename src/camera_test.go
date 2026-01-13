//go:build linux

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

func main() {
	fmt.Println("=== Camera Diagnostic Tool ===")

	// 1. List all video devices
	matches, err := filepath.Glob("/sys/class/video4linux/video*")
	if err != nil {
		log.Fatalf("Failed to glob video devices: %v", err)
	}

	if len(matches) == 0 {
		fmt.Println("No video devices found in /sys/class/video4linux/")
		return
	}

	for _, match := range matches {
		devPath := "/dev/" + filepath.Base(match)
		namePath := filepath.Join(match, "name")
		nameBytes, _ := os.ReadFile(namePath)
		name := strings.TrimSpace(string(nameBytes))

		fmt.Printf("\nChecking Device: %s (%s)\n", devPath, name)

		// Try to open and capture
		if err := testDevice(devPath); err != nil {
			fmt.Printf("  [!] Failed: %v\n", err)
		} else {
			fmt.Printf("  [+] Success! Frame saved for %s\n", devPath)
		}
	}
}

func testDevice(devName string) error {
	fmt.Printf("  Opening %s...\n", devName)

	// Try MJPEG first
	cam, err := device.Open(
		devName,
		device.WithPixFormat(v4l2.PixFormat{
			PixelFormat: v4l2.PixelFmtMJPEG,
			Width:       640,
			Height:      480,
		}),
	)

	if err != nil {
		fmt.Printf("  MJPEG open failed, trying default format...\n")
		cam, err = device.Open(devName)
		if err != nil {
			return fmt.Errorf("open failed: %w", err)
		}
	}
	defer cam.Close()

	// List supported formats for debugging
	caps := cam.Capability()
	fmt.Printf("  Driver: %s, Card: %s, Bus: %s\n", caps.Driver, caps.Card, caps.BusInfo)

	// Start stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Capture one frame
	fmt.Println("  Waiting for frame...")
	frames := cam.GetFrames()

	if err := cam.Start(ctx); err != nil {
		return fmt.Errorf("start stream failed: %w", err)
	}
	defer cam.Stop()

	select {
	case frame, ok := <-frames:
		if !ok {
			return fmt.Errorf("frame channel closed")
		}
		defer frame.Release()

		fileName := fmt.Sprintf("test_frame_%s.jpg", filepath.Base(devName))
		if err := os.WriteFile(fileName, frame.Data, 0644); err != nil {
			return fmt.Errorf("failed to save frame: %w", err)
		}
		fmt.Printf("  Captured %d bytes to %s\n", len(frame.Data), fileName)
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("timeout waiting for frame")
	}
}
