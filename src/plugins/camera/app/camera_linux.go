//go:build linux && cgo

package camera

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
	"dialtone/cli/src/core/logger"
)

// Log wrappers to match dialtone logger if needed, or import standard logger
func LogInfo(format string, args ...interface{}) {
	logger.LogInfo(format, args...)
}

// Camera info structure
type Camera struct {
	Device string `json:"device"`
	Name   string `json:"name"`
}

// ListCameras scans /sys/class/video4linux to find connected video devices.
func ListCameras() ([]Camera, error) {
	var cameras []Camera
	matches, err := filepath.Glob("/sys/class/video4linux/video*")
	if err != nil {
		return nil, fmt.Errorf("failed to list video devices: %w", err)
	}

	for _, match := range matches {
		device := "/dev/" + filepath.Base(match)
		namePath := filepath.Join(match, "name")
		nameBytes, err := os.ReadFile(namePath)
		name := "Unknown Camera"
		if err == nil {
			name = strings.TrimSpace(string(nameBytes))
		}

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
	camDev        *device.Device
	camMu         sync.Mutex // Mutex for Start/Stop lifecycle
	camWg         sync.WaitGroup
	latestFrame   []byte
	frameMu       sync.RWMutex // Mutex for frame buffer
	lastFrameTime time.Time
	camCancel     context.CancelFunc // To stop the capture loop
)

// StartCamera initializes the camera if not already started.
func StartCamera(ctx context.Context, devName string) error {
	camMu.Lock()
	defer camMu.Unlock()

	if camDev != nil {
		return nil // Already running
	}

	// Wait for any previous shutdown to complete just in case
	camWg.Wait()

	// Persistent context for the capture loop
	loopCtx, cancel := context.WithCancel(context.Background())
	camCancel = cancel

	LogInfo("Opening camera device %s...", devName)

	useYUYV := os.Getenv("DIALTONE_CAMERA_FORMAT") == "yuyv"
	
	// Configure Format
	format := v4l2.PixelFmtMJPEG
	if useYUYV {
		format = v4l2.PixelFmtYUYV
		LogInfo("Configured for YUYV format (Software Encoding)")
	}

	// ATOMIC OPEN PATTERN (Matches go4vl examples)
	// We explicity set IOTypeMMAP and BufferSize to ensure stability
	cam, err := device.Open(
		devName,
		device.WithIOType(v4l2.IOTypeMMAP),
		device.WithPixFormat(v4l2.PixFormat{
			PixelFormat: format,
			Width:       640,
			Height:      480,
		}),
		device.WithBufferSize(4), // Use 4 buffers for smooth streaming
	)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to open device: %w", err)
	}

	// Use GetOutput() which handles frame release/copy for us. 
	// IMPORTANT: Must be called BEFORE Start() to select the streaming API (MMAP)
	frames := cam.GetOutput()

	// With Atomic Open + MMAP + GetOutput called, we can now Start()
	if err := cam.Start(loopCtx); err != nil {
		cam.Close()
		cancel()
		return fmt.Errorf("failed to start stream: %w", err)
	}

	camDev = cam
	camWg.Add(1) // Register the capture loop

	// Start background capture loop
	go captureLoop(loopCtx, cam, frames, useYUYV)

	return nil
}

// StopCamera stops the camera capture loop and releases the device.
func StopCamera() {
	camMu.Lock()
	cancel := camCancel
	// Don't nil pointers yet, let the loop do it or on restart
	camMu.Unlock()

	if cancel != nil {
		cancel() // Signal loop to exit
	}
	
	// Wait for loop to fully exit and close device
	camWg.Wait()
}

// captureLoop runs in the background and updates the global latestFrame buffer
func captureLoop(ctx context.Context, cam *device.Device, frames <-chan []byte, useYUYV bool) {
	defer func() {
		camMu.Lock()
		if camDev == cam {
			camDev = nil
			camCancel = nil
		}
		camMu.Unlock()
		cam.Close()
		camWg.Done() // Signal shutdown complete
		LogInfo("Camera capture loop stopped")
	}()

	frameCount := 0
	lastLog := time.Now()

	LogInfo("Camera capture loop started")

	for {
		select {
		case <-ctx.Done():
			return
		case frameData, ok := <-frames:
			if !ok {
				LogInfo("Camera frame channel closed")
				return
			}

			// Diagnostic logging
			frameCount++
			if frameCount <= 5 || time.Since(lastLog) > 5*time.Second {
				LogInfo("Camera capturing: Frame #%d (Len: %d, YUYV: %v)", frameCount, len(frameData), useYUYV)
				lastLog = time.Now()
			}

			var imgData []byte
			
			if useYUYV {
				width, height := 640, 480
				if len(frameData) >= width*height*2 {
					jpgData, err := yuyvToJpeg(frameData, width, height)
					if err == nil {
						imgData = jpgData
					} else {
						// throttle error logging
						if frameCount % 30 == 0 {
							LogInfo("Error encoding jpeg: %v", err)
						}
					}
				}
			} else {
				// MJPEG Pass-through
				// frameData from GetOutput is likely a copy or valid until next get?
				// go4vl GetOutput() returns a channel of []byte. 
				// NOTE: go4vl implementation copies data from MMap buffer to a new slice before sending to channel.
				// So it is safe to hold this reference.
				imgData = frameData
			}

			if imgData != nil {
				frameMu.Lock()
				latestFrame = imgData
				lastFrameTime = time.Now()
				frameMu.Unlock()
			}
		}
	}
}

// Simple YUYV to JPEG converter
func yuyvToJpeg(data []byte, width, height int) ([]byte, error) {
	rect := image.Rect(0, 0, width, height)
	img := &image.YCbCr{
		Y:              make([]uint8, width*height),
		Cb:             make([]uint8, width*height/2),
		Cr:             make([]uint8, width*height/2),
		SubsampleRatio: image.YCbCrSubsampleRatio422,
		YStride:        width,
		CStride:        width / 2,
		Rect:           rect,
	}

	j := 0
	k := 0
	for i := 0; i < len(data) && i+3 < len(data); i += 4 {
		y0, u, y1, v := data[i], data[i+1], data[i+2], data[i+3]

		img.Y[j] = y0
		img.Y[j+1] = y1
		j += 2

		img.Cb[k] = u
		img.Cr[k] = v
		k++
	}

	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 75})
	return buf.Bytes(), err
}

// StreamHandler handles MJPEG streaming requests using the latestFrame buffer.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Auto-start camera if needed
	camMu.Lock()
	if camDev == nil {
		camMu.Unlock() 
		
		cameras, err := ListCameras()
		if err == nil && len(cameras) > 0 {
			if err := StartCamera(r.Context(), cameras[0].Device); err != nil {
				LogInfo("Failed to auto-start camera: %v", err)
			}
		} else {
			LogInfo("No cameras found during auto-start")
		}
	} else {
		camMu.Unlock()
	}

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, hasFlusher := w.(http.Flusher)
	if !hasFlusher {
		LogInfo("Warning: Client does not support http.Flusher")
	}

	LogInfo("Starting stream for %s", r.RemoteAddr)

	ticker := time.NewTicker(40 * time.Millisecond) // ~25 fps
	defer ticker.Stop()

	var lastSent time.Time

	for {
		select {
		case <-r.Context().Done():
			LogInfo("Stream closed by client %s", r.RemoteAddr)
			return
		case <-ticker.C:
			frameMu.RLock()
			data := latestFrame
			ts := lastFrameTime
			frameMu.RUnlock()

			if data == nil || ts.Equal(lastSent) {
				continue
			}
			
			lastSent = ts

			// Write Header
			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(data))
			if err != nil {
				return
			}
			
			// Write Data
			if _, err := w.Write(data); err != nil {
				return
			}

			// Write Boundary
			if _, err := w.Write([]byte("\r\n")); err != nil {
				return
			}

			// Flush
			if hasFlusher {
				flusher.Flush()
			}
		}
	}
}

// GetLatestFrame returns the most recent frame buffer and its timestamp
func GetLatestFrame() ([]byte, time.Time) {
	frameMu.RLock()
	defer frameMu.RUnlock()
	
	if latestFrame == nil {
		return nil, time.Time{}
	}
	
	// Return a copy to be safe
	buf := make([]byte, len(latestFrame))
	copy(buf, latestFrame)
	return buf, lastFrameTime
}
