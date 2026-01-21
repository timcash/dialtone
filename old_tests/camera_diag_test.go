//go:build linux

package test

import (
	"context"
	dialtone "dialtone/cli/src"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

func TestCameraDiag(t *testing.T) {
	dialtone.LogInfo("=== Camera Diagnostic Tool ===")

	// 1. List all video devices
	matches, err := filepath.Glob("/sys/class/video4linux/video*")
	if err != nil {
		dialtone.LogFatal("Failed to glob video devices: %v", err)
	}

	if len(matches) == 0 {
		dialtone.LogInfo("No video devices found in /sys/class/video4linux/")
		return
	}

	for _, match := range matches {
		devPath := "/dev/" + filepath.Base(match)
		namePath := filepath.Join(match, "name")
		nameBytes, _ := os.ReadFile(namePath)
		name := strings.TrimSpace(string(nameBytes))

		dialtone.LogPrintf("Checking Device: %s (%s)", devPath, name)

		// Try to open and capture
		if err := testDevice(devPath); err != nil {
			dialtone.LogPrintf("  [!] Failed: %v", err)
		} else {
			dialtone.LogPrintf("  [+] Success! Frame saved for %s", devPath)
		}
	}
}

func testDevice(devName string) error {
	dialtone.LogPrintf("  Opening %s...", devName)

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
		dialtone.LogPrintf("  MJPEG open failed, trying default format...")
		cam, err = device.Open(devName)
		if err != nil {
			return fmt.Errorf("open failed: %w", err)
		}
	}
	defer cam.Close()

	// List supported formats for debugging
	caps := cam.Capability()
	dialtone.LogPrintf("  Driver: %s, Card: %s, Bus: %s", caps.Driver, caps.Card, caps.BusInfo)

	// Start stream
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Capture one frame
	dialtone.LogInfo("  Waiting for frame...")
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
		dialtone.LogPrintf("  Captured %d bytes to %s", len(frame.Data), fileName)
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("timeout waiting for frame")
	}
}
