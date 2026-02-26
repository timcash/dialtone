package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

func main() {
	logs.SetOutput(os.Stdout)
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "version":
		logs.Raw("camera_v1")
	case "run":
		if err := run(os.Args[2:]); err != nil {
			logs.Error("camera run failed: %v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		logs.Error("unknown command: %s", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS URL")
	subject := fs.String("subject", "camera.heartbeat", "Heartbeat subject")
	interval := fs.Duration("interval", time.Second, "Publish interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	nc, err := nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(2*time.Second))
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	logs.Info("camera_v1 heartbeat publisher started subject=%s nats=%s", *subject, *natsURL)
	for {
		select {
		case <-ctx.Done():
			logs.Info("camera_v1 stopping")
			return nil
		case t := <-ticker.C:
			msg := fmt.Sprintf(`{"source":"camera_v1","ts":"%s"}`, t.UTC().Format(time.RFC3339Nano))
			if err := nc.Publish(*subject, []byte(msg)); err != nil {
				logs.Warn("camera_v1 publish error: %v", err)
				continue
			}
			_ = nc.Flush()
		}
	}
}

func usage() {
	logs.Raw("Usage: dialtone_camera_v1 <command>")
	logs.Raw("Commands:")
	logs.Raw("  run [--nats-url URL] [--subject camera.heartbeat] [--interval 1s]")
	logs.Raw("  version")
}
