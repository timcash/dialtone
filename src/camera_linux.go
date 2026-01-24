//go:build linux && cgo

package dialtone

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
)

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

	// Create a persistent context for the capture loop (detached from the request)
	// This ensures the camera keeps running even if the initial requester disconnects
	loopCtx, cancel := context.WithCancel(context.Background())
	camCancel = cancel

	LogInfo("Opening camera device %s...", devName)

	// Check environment for format override
	useYUYV := os.Getenv("DIALTONE_CAMERA_FORMAT") == "yuyv"

	// Default to MJPEG
	format := v4l2.PixelFmtMJPEG
	if useYUYV {
		format = v4l2.PixelFmtYUYV
		LogInfo("Configured for YUYV format (Software Encoding)")
	}

	// Initialize with Split Open/SetFormat to avoid some atomic open issues
	cam, err := device.Open(devName)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to open device: %w", err)
	}

	if err := cam.SetPixFormat(v4l2.PixFormat{
		PixelFormat: format,
		Width:       640,
		Height:      480,
	}); err != nil {
		cam.Close()
		cancel()
		return fmt.Errorf("failed to set format: %w", err)
	}

	// go4vl requirement: call GetFrames before Start to setup buffers or select method
	// We will do this inside the capture loop logic or just here.
	// Actually, best to just Start() and let Go4vl handle it, but sometimes GetFrames is needed first.
	// We'll call GetFrames in the loop initialization.

	if err := cam.Start(loopCtx); err != nil {
		cam.Close()
		cancel()
		return fmt.Errorf("failed to start stream: %w", err)
	}

	camDev = cam

	// Start background capture loop
	go captureLoop(loopCtx, cam, useYUYV)

	return nil
}

// captureLoop runs in the background and updates the global latestFrame buffer
func captureLoop(ctx context.Context, cam *device.Device, useYUYV bool) {
	defer func() {
		camMu.Lock()
		if camDev == cam {
			camDev = nil
		}
		camMu.Unlock()
		cam.Close()
		LogInfo("Camera capture loop stopped")
	}()

	frames := cam.GetFrames()
	frameCount := 0
	lastLog := time.Now()

	LogInfo("Camera capture loop started")

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-frames:
			if !ok {
				LogInfo("Camera frame channel closed")
				return
			}

			// Diagnostic logging
			frameCount++
			if time.Since(lastLog) > 5*time.Second {
				LogInfo("Camera capturing: %d frames in last 5s (Len: %d, YUYV: %v)", frameCount, len(frame.Data), useYUYV)
				frameCount = 0
				lastLog = time.Now()
			}

			var imgData []byte
			
			if useYUYV {
				// Convert YUYV 4:2:2 to JPEG
				// Data is Y0 U0 Y1 V0 ...
				width, height := 640, 480
				if len(frame.Data) >= width*height*2 {
					jpgData, err := yuyvToJpeg(frame.Data, width, height)
					if err == nil {
						imgData = jpgData
					} else {
						LogInfo("Error encoding jpeg: %v", err)
					}
				}
			} else {
				// MJPEG Pass-through
				// Clone only if necessary, but here we need to persist it in global var
				imgData = make([]byte, len(frame.Data))
				copy(imgData, frame.Data)
			}

			if imgData != nil {
				frameMu.Lock()
				latestFrame = imgData
				lastFrameTime = time.Now()
				frameMu.Unlock()
			}

			frame.Release()
		}
	}
}

// Simple YUYV to JPEG converter
// YUYV 4:2:2 is [Y0, U0, Y1, V0]
// YCbCr 4:2:2 implies chroma is shared between two pixels
func yuyvToJpeg(data []byte, width, height int) ([]byte, error) {
	rect := image.Rect(0, 0, width, height)
	// Create YCbCr image. Default is 4:4:4 in Go usually? 
	// image.YCbCr structure supports SubsampleRatio.
	img := &image.YCbCr{
		Y:              make([]uint8, width*height),
		Cb:             make([]uint8, width*height/2),
		Cr:             make([]uint8, width*height/2),
		SubsampleRatio: image.YCbCrSubsampleRatio422,
		YStride:        width,
		CStride:        width / 2,
		Rect:           rect,
	}

	// Y0 U0 Y1 V0
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
	// Quality 75 is standard
	err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 75})
	return buf.Bytes(), err
}

// StreamHandler handles MJPEG streaming requests using the latestFrame buffer.
// This allows multiple viewers without contending for the camera callbacks.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	// Auto-start camera if needed
	camMu.Lock()
	if camDev == nil {
		camMu.Unlock() // Unlock to allow StartCamera to acquire lock
		
		// Find a camera
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

	ticker := time.NewTicker(40 * time.Millisecond) // ~25 fps target for stream
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

			// Don't resend the same frame (deduplication)
			if data == nil || ts.Equal(lastSent) {
				continue
			}

			// Wait until we have a recent frame (optional staleness check could go here)
			
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

			// Flush immediately to prevent buffering in browser or intermediaries
			if hasFlusher {
				flusher.Flush()
			}
		}
	}
}
