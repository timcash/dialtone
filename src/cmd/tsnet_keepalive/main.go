package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	tsnetv1 "dialtone/dev/plugins/tsnet/src_v1/go"
)

func main() {
	logs.SetOutput(os.Stdout)

	hostname := flag.String("hostname", "", "Override tsnet hostname (defaults to DIALTONE_HOSTNAME)")
	stateDir := flag.String("state-dir", "", "Override tsnet state dir (defaults to DIALTONE_TSNET_STATE_DIR/.dialtone/tsnet)")
	flag.Parse()

	cfg, err := tsnetv1.ResolveConfig(*hostname, *stateDir)
	if err != nil {
		logs.Error("tsnet keepalive config failed: %v", err)
		os.Exit(1)
	}

	srv := tsnetv1.BuildServer(cfg)
	srv.Ephemeral = false

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st, err := srv.Up(ctx)
	if err != nil {
		logs.Error("tsnet keepalive up failed: %v", err)
		os.Exit(1)
	}

	ip4, ip6 := srv.TailscaleIPs()
	logs.Info("tsnet keepalive online: backend=%s dns=%s ip4=%s ip6=%s hostname=%s tailnet=%s",
		st.BackendState, st.Self.DNSName, ip4.String(), ip6.String(), cfg.Hostname, cfg.Tailnet)

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	if err := srv.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "close error: %v\n", err)
	}
}
