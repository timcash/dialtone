package cloudflare

import (
	"fmt"
	"os"
	"os/exec"
)

func BuildTunnelRunCommand(cloudflaredBin, name, url, token string) (*exec.Cmd, error) {
	if cloudflaredBin == "" {
		return nil, fmt.Errorf("cloudflared binary is required")
	}
	args, err := BuildTunnelRunArgs(name, url, token)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(cloudflaredBin, append([]string{"tunnel"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd, nil
}
