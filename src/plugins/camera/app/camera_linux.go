//go:build linux

package camera

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dialtone/dev/plugins/logs/src_v1/go"
	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

// Log wrappers to match dialtone logger
func LogInfo(format string, args ...interface{}) {
	logs.Info(format, args...)
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
	camDev               *device.Device
	camFrames            <-chan *device.Frame
	camMu                sync.Mutex
	lastAutoStartAttempt time.Time
	lastAutoStartLogAt   time.Time
	autoStartBackoff     = 2 * time.Second
	autoStartLogThrottle = 10 * time.Second
)

// StartCamera initializes the camera if not already started.
func StartCamera(ctx context.Context, devName string) error {
	camMu.Lock()
	defer camMu.Unlock()

	if camDev != nil {
		return nil
	}

	LogInfo("Opening camera device %s...", devName)
	cam, err := openCameraDevice(devName)
	if err != nil {
		return fmt.Errorf("failed to open device: %w", err)
	}

	// Select frame-stream mode before Start(). go4vl creates channels at Start().
	_ = cam.GetFrames()

	if err := cam.Start(ctx); err != nil {
		cam.Close()
		return fmt.Errorf("failed to start stream: %w", err)
	}
	frames := cam.GetFrames()
	if frames == nil {
		cam.Close()
		return fmt.Errorf("camera frame channel unavailable after start")
	}

	camDev = cam
	camFrames = frames
	LogInfo("Camera stream mode: frames")
	go func(devName string, errs <-chan error) {
		for err := range errs {
			LogInfo("Camera stream error on %s: %v", devName, err)
		}
	}(devName, cam.GetError())
	return nil
}

func openCameraDevice(devName string) (*device.Device, error) {
	type openProfile struct {
		name    string
		options []device.Option
	}
	profiles := []openProfile{
		{
			name: "mjpeg-640x480",
			options: []device.Option{
				device.WithPixFormat(v4l2.PixFormat{
					PixelFormat: v4l2.PixelFmtMJPEG,
					Width:       640,
					Height:      480,
				}),
			},
		},
		{
			name: "mjpeg-1280x720",
			options: []device.Option{
				device.WithPixFormat(v4l2.PixFormat{
					PixelFormat: v4l2.PixelFmtMJPEG,
					Width:       1280,
					Height:      720,
				}),
			},
		},
		{
			name: "yuyv-640x480",
			options: []device.Option{
				device.WithPixFormat(v4l2.PixFormat{
					PixelFormat: v4l2.PixelFmtYUYV,
					Width:       640,
					Height:      480,
				}),
			},
		},
		{
			name:    "device-default",
			options: nil,
		},
	}
	var lastErr error
	for _, p := range profiles {
		cam, err := device.Open(devName, p.options...)
		if err == nil {
			LogInfo("Camera open profile selected: %s", p.name)
			return cam, nil
		}
		lastErr = err
		LogInfo("Camera open profile %s failed on %s: %v", p.name, devName, err)
	}
	return nil, lastErr
}

// StopCamera cleans up the camera device
// Added for compatibility with other plugins that might expect this
func StopCamera() {
	camMu.Lock()
	defer camMu.Unlock()
	if camDev != nil {
		camDev.Close()
		camDev = nil
		camFrames = nil
	}
}

// GetLatestFrame returns a nil frame for now, as this method does not keep a buffer
// Added for compatibility with diagnostic checks if they remain
func GetLatestFrame() ([]byte, time.Time) {
	return nil, time.Time{}
}

// StreamHandler handles MJPEG streaming requests.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Auto-start logic with backoff to avoid hot-loop retries when camera is unavailable.
	camMu.Lock()
	if camDev == nil {
		now := time.Now()
		if now.Sub(lastAutoStartAttempt) < autoStartBackoff {
			camMu.Unlock()
		} else {
			lastAutoStartAttempt = now
			camMu.Unlock()
			candidates := candidateDevices()
			var lastErr error
			if len(candidates) == 0 && now.Sub(lastAutoStartLogAt) >= autoStartLogThrottle {
				LogInfo("No usable camera capture devices found")
				lastAutoStartLogAt = now
			}
			for _, devName := range candidates {
				LogInfo("Auto-starting camera %s...", devName)
				// Keep camera lifetime independent from individual HTTP request lifecycle.
				if err := StartCamera(context.Background(), devName); err != nil {
					LogInfo("Auto-start failed for %s: %v", devName, err)
					lastErr = err
					continue
				}
				lastErr = nil
				break
			}
			if lastErr != nil && now.Sub(lastAutoStartLogAt) >= autoStartLogThrottle {
				LogInfo("Failed to auto-start camera from candidates %v: %v", candidates, lastErr)
				lastAutoStartLogAt = now
			}
		}
	} else {
		camMu.Unlock()
	}

	camMu.Lock()
	cam := camDev
	frames := camFrames
	camMu.Unlock()

	if cam == nil || frames == nil {
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

	tryRestart := func() bool {
		StopCamera()
		for _, devName := range candidateDevices() {
			LogInfo("Attempting camera restart on %s...", devName)
			if err := StartCamera(context.Background(), devName); err != nil {
				LogInfo("Camera restart failed on %s: %v", devName, err)
				continue
			}
			camMu.Lock()
			cam = camDev
			frames = camFrames
			camMu.Unlock()
			if cam != nil && frames != nil {
				return true
			}
		}
		return false
	}

	for {
		select {
		case <-r.Context().Done():
			LogInfo("Stream closed by client %s", r.RemoteAddr)
			return
		case frame, ok := <-frames:
			if !ok {
				LogInfo("Frames channel closed")
				if tryRestart() {
					continue
				}
				return
			}
			// Write the MJPEG boundary and frame metadata
			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame.Data))
			if err != nil {
				LogInfo("Stream write error (header) for %s: %v", r.RemoteAddr, err)
				frame.Release()
				return
			}

			// Write the actual JPEG data
			if _, err := w.Write(frame.Data); err != nil {
				LogInfo("Stream write error (body) for %s: %v", r.RemoteAddr, err)
				frame.Release()
				return
			}

			_, _ = w.Write([]byte("\r\n"))
			flusher.Flush()
			frame.Release()
		}
	}
}

func candidateDevices() []string {
	seen := map[string]struct{}{}
	var candidates []string
	// Explicit override first.
	if manual := strings.TrimSpace(os.Getenv("CAMERA_DEVICE")); manual != "" {
		out := []string{manual}
		// Common USB webcams expose capture on video0 or video1 depending on boot order.
		if strings.HasPrefix(filepath.Base(manual), "video") {
			if strings.HasSuffix(manual, "0") {
				out = append(out, strings.TrimSuffix(manual, "0")+"1")
			} else if strings.HasSuffix(manual, "1") {
				out = append(out, strings.TrimSuffix(manual, "1")+"0")
			}
		}
		return out
	}

	cameras, err := ListCameras()
	if err == nil {
		for _, cam := range cameras {
			dev := strings.TrimSpace(cam.Device)
			if dev == "" {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(cam.Name))
			// Skip known non-capture blocks.
			if strings.Contains(name, "codec") ||
				strings.Contains(name, "isp") ||
				strings.Contains(name, "hevc") ||
				strings.Contains(name, "decode") ||
				strings.Contains(name, "encode") ||
				strings.Contains(name, "image_fx") ||
				strings.Contains(name, "stats") {
				continue
			}
			if _, ok := seen[dev]; ok {
				continue
			}
			seen[dev] = struct{}{}
			candidates = append(candidates, dev)
		}
	}

	if len(candidates) == 0 {
		// Fallback deterministic scan over low video indexes (commonly capture devices).
		matches, _ := filepath.Glob("/dev/video*")
		sort.Slice(matches, func(i, j int) bool {
			return videoIndex(matches[i]) < videoIndex(matches[j])
		})
		for _, dev := range matches {
			idx := videoIndex(dev)
			if idx < 0 || idx > 9 {
				continue
			}
			if _, ok := seen[dev]; ok {
				continue
			}
			seen[dev] = struct{}{}
			candidates = append(candidates, dev)
		}
	}
	return candidates
}

func videoIndex(dev string) int {
	base := filepath.Base(dev)
	if !strings.HasPrefix(base, "video") {
		return 1 << 30
	}
	n, err := strconv.Atoi(strings.TrimPrefix(base, "video"))
	if err != nil {
		return 1 << 30
	}
	return n
}
