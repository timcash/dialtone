//go:build linux

package dialtone

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

// Camera info structure
type Camera struct {
	Device string `json:"device"`
	Name   string `json:"name"`
}

// ListCameras scans /sys/class/video4linux to find connected video devices.
// This works on most Linux systems including Raspberry Pi.
func ListCameras() ([]Camera, error) {
	var cameras []Camera

	// Find all video devices in /sys/class/video4linux
	matches, err := filepath.Glob("/sys/class/video4linux/video*")
	if err != nil {
		return nil, fmt.Errorf("failed to list video devices: %w", err)
	}

	for _, match := range matches {
		device := "/dev/" + filepath.Base(match)

		// Read the name of the device
		namePath := filepath.Join(match, "name")
		nameBytes, err := os.ReadFile(namePath)
		name := "Unknown Camera"
		if err == nil {
			name = strings.TrimSpace(string(nameBytes))
		}

		// Check if it's a capture device (some devices have multiple entries, e.g. metadata)
		// We can check /sys/class/video4linux/videoX/index or other attributes
		// For simplicity, we filter out common non-capture devices like 'bcm2835-isp'
		if strings.Contains(strings.ToLower(name), "isp") || strings.Contains(strings.ToLower(name), "codec") {
			continue
		}

		cameras = append(cameras, Camera{
			Device: device,
			Name:   name,
		})
	}

	return cameras, nil
}

var (
	camDev  *device.Device
	camMu   sync.Mutex
	camOnce sync.Once
)

// StartCamera initializes the camera if not already started.
func StartCamera(ctx context.Context, devName string) error {
	camMu.Lock()
	defer camMu.Unlock()

	if camDev != nil {
		return nil
	}

	LogInfo("Opening camera device %s...", devName)
	cam, err := device.Open(
		devName,
		device.WithPixFormat(v4l2.PixFormat{
			PixelFormat: v4l2.PixelFmtMJPEG,
			Width:       640,
			Height:      480,
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}

	// Requirement for go4vl: GetFrames() or similar must be called before Start()
	// to select the streaming API. We do this by getting the frame channel.
	_ = cam.GetFrames()

	if err := cam.Start(ctx); err != nil {
		cam.Close()
		return fmt.Errorf("failed to start stream: %w", err)
	}

	camDev = cam
	return nil
}

// StreamHandler handles MJPEG streaming requests.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	camMu.Lock()
	cam := camDev
	camMu.Unlock()

	if cam == nil {
		http.Error(w, "Camera not initialized", http.StatusServiceUnavailable)
		return
	}

	// Set headers for MJPEG
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	LogInfo("Starting stream for %s", r.RemoteAddr)

	// Get the frame channel from the camera
	frames := cam.GetFrames()

	for {
		select {
		case <-r.Context().Done():
			LogInfo("Stream closed by client %s", r.RemoteAddr)
			return
		case frame, ok := <-frames:
			if !ok {
				LogInfo("Frame channel closed")
				return
			}
			// Write the MJPEG boundary and frame metadata
			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame.Data))
			if err != nil {
				frame.Release()
				return // Client disconnected
			}

			// Write the actual JPEG data
			if _, err := w.Write(frame.Data); err != nil {
				frame.Release()
				return
			}

			_, _ = w.Write([]byte("\r\n"))

			// Important: Release the frame back to the pool
			frame.Release()
		}
	}
}
