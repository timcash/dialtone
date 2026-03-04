package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"tailscale.com/tsnet"
)

type keepaliveConfig struct {
	hostname string
	stateDir string
}

func runKeepalive(args []string) error {
	opts := parseKeepaliveArgs(args)

	repoRoot, err := locateRepoRoot()
	if err != nil {
		return err
	}
	cfg, err := resolveConfig(opts.hostname, opts.stateDir)
	if err != nil {
		return err
	}

	stateDir := strings.TrimSpace(cfg.StateDir)
	if strings.TrimSpace(opts.stateDir) != "" {
		stateDir = strings.TrimSpace(opts.stateDir)
	}
	if !filepath.IsAbs(stateDir) {
		stateDir = filepath.Join(repoRoot, stateDir)
	}

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("init tsnet state dir: %w", err)
	}
	cfg.StateDir = stateDir
	cfg.Hostname = sanitizeHost(cfg.Hostname)
	if cfg.Hostname == "" {
		return fmt.Errorf("resolved empty hostname")
	}

	authKey := strings.TrimSpace(os.Getenv(cfg.AuthKeyEnv))
	if authKey == "" {
		return fmt.Errorf("%s missing; run 'tsnet v1 bootstrap' first", cfg.AuthKeyEnv)
	}

	srv := &tsnet.Server{
		Hostname: cfg.Hostname,
		Dir:      cfg.StateDir,
		AuthKey:  authKey,
		Logf: func(format string, args ...any) {
			fmt.Printf("[tsnet] "+format+"\n", args...)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	st, err := srv.Up(ctx)
	if err != nil {
		return fmt.Errorf("tsnet keepalive up failed: %w", err)
	}

	backend := "-"
	dnsName := "-"
	if st != nil {
		if strings.TrimSpace(st.BackendState) != "" {
			backend = st.BackendState
		}
		if st.Self != nil && strings.TrimSpace(st.Self.DNSName) != "" {
			dnsName = st.Self.DNSName
		}
	}

	ip4, ip6 := srv.TailscaleIPs()
	fmt.Printf("tsnet keepalive online: backend=%s dns=%s ip4=%s ip6=%s hostname=%s tailnet=%s\n",
		backend,
		dnsName,
		ip4.String(),
		ip6.String(),
		cfg.Hostname,
		cfg.Tailnet,
	)

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	fmt.Fprintln(os.Stderr, "stopping tsnet keepalive")
	if err := srv.Close(); err != nil {
		return err
	}
	return nil
}

func parseKeepaliveArgs(args []string) keepaliveConfig {
	cfg := keepaliveConfig{
		hostname: sanitizeHost(os.Getenv("DIALTONE_HOSTNAME")),
		stateDir: "",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--host":
			if i+1 < len(args) {
				cfg.hostname = sanitizeHost(args[i+1])
				i++
			}
		case "--state-dir":
			if i+1 < len(args) {
				cfg.stateDir = strings.TrimSpace(args[i+1])
				i++
			}
		}
	}
	return cfg
}
