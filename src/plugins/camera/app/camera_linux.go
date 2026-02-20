//go:build linux && cgo

package camera

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dialtone/dev/logger"
	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

// Log wrappers to match dialtone logger
func LogInfo(format string, args ...interface{}) {
	logger.LogInfo(format, args...)
}

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
	camDev *device.Device
	camMu  sync.Mutex
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

// StopCamera cleans up the camera device
// Added for compatibility with other plugins that might expect this
func StopCamera() {
	camMu.Lock()
	defer camMu.Unlock()
	if camDev != nil {
		camDev.Close()
		camDev = nil
	}
}

// GetLatestFrame returns a nil frame for now, as this method does not keep a buffer
// Added for compatibility with diagnostic checks if they remain
func GetLatestFrame() ([]byte, time.Time) {
	return nil, time.Time{}
}

// StreamHandler handles MJPEG streaming requests.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Auto-start logic (Added to ensure user request works)
	camMu.Lock()
	if camDev == nil {
		camMu.Unlock()
		cameras, err := ListCameras()
		if err == nil && len(cameras) > 0 {
			LogInfo("Auto-starting camera %s...", cameras[0].Device)
			// Create a background context for the camera so it doesn't die when this request dies
			// Note: Ideally we manage lifecycle better, but for this revert we match 'it just works' behavior
			if err := StartCamera(context.Background(), cameras[0].Device); err != nil {
				LogInfo("Failed to auto-start camera: %v", err)
			}
		} else {
			LogInfo("No cameras found for auto-start")
		}
	} else {
		camMu.Unlock()
	}

	camMu.Lock()
	cam := camDev
	camMu.Unlock()

	if cam == nil {
		http.Error(w, "Camera not initialized", http.StatusServiceUnavailable)
		return
	}

	// Set headers for MJPEG
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, pre-check=0, post-check=0, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Connection", "close")

	flusher, ok := w.(http.Flusher)
	if !ok {
		LogInfo("ResponseWriter does not support flushing")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	LogInfo("Starting stream for %s", r.RemoteAddr)
	flusher.Flush()

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
				LogInfo("Stream write error (header) for %s: %v", r.RemoteAddr, err)
				if r, ok := interface{}(frame).(interface{ Release() }); ok {
					r.Release()
				}
				return
			}

			// Write the actual JPEG data
			if _, err := w.Write(frame.Data); err != nil {
				LogInfo("Stream write error (body) for %s: %v", r.RemoteAddr, err)
				if r, ok := interface{}(frame).(interface{ Release() }); ok {
					r.Release()
				}
				return
			}

			_, _ = w.Write([]byte("\r\n"))
			flusher.Flush()

			// Important: Release the frame back to the pool
			if r, ok := interface{}(frame).(interface{ Release() }); ok {
				r.Release()
			}
		}
	}
}
