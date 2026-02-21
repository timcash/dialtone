package main

import (
	"fmt"

	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func main() {
	cfg, err := tsnetv1.ResolveConfig("Robot_1 Dev", ".dialtone/tsnet-example")
	if err != nil {
		panic(err)
	}
	if cfg.Hostname != "robot-1-dev" {
		panic("hostname normalization failed")
	}

	srv := tsnetv1.BuildServer(cfg)
	if srv.Hostname != "robot-1-dev" {
		panic("server hostname mismatch")
	}

	usages := tsnetv1.InferKeyUsage(
		[]tsnetv1.AuthKey{
			{ID: "k1", Description: "robot-1-dev", Tags: []string{"tag:robot"}},
		},
		[]tsnetv1.Device{
			{Name: "robot-1-dev", Hostname: "robot-1-dev", Tags: []string{"tag:robot"}},
		},
	)
	if len(usages) != 1 || len(usages[0].Matches) == 0 {
		panic("key usage inference mismatch")
	}

	fmt.Println("TSNET_LIBRARY_EXAMPLE_PASS")
}
