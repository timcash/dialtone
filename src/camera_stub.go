//go:build !linux

package dialtone

import (
	"context"
	"net/http"
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

// StreamHandler returns a not implemented error on non-linux platforms.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Camera streaming is only supported on Linux", http.StatusNotImplemented)
}
