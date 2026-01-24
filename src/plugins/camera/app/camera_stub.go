//go:build !linux || (linux && !cgo)

package camera

import (
	"context"
	"net/http"
	"time"
)

// Camera info structure
type Camera struct {
	Device string `json:"device"`
	Name   string `json:"name"`
}

// ListCameras returns an empty list on non-linux platforms.
func ListCameras() ([]Camera, error) {
	return []Camera{}, nil
}

// StartCamera is a no-op on non-linux platforms.
func StartCamera(ctx context.Context, devName string) error {
	return nil
}

// StopCamera is a no-op on non-linux platforms.
func StopCamera() {
}

// StreamHandler returns a not implemented error on non-linux platforms.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Camera streaming is only supported on Linux", http.StatusNotImplemented)
}

// GetLatestFrame returns nil on non-linux platforms.
func GetLatestFrame() ([]byte, time.Time) {
	return nil, time.Time{}
}
