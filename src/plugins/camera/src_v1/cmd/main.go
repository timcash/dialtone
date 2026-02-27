package main

import (
	"context"
	cameraapp "dialtone/dev/plugins/camera/app"
	"flag"
	"fmt"
	"net/http"
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
	listen := fs.String("listen", ":19090", "HTTP listen address for stream service")
	serveStream := fs.Bool("serve-stream", true, "Expose /stream endpoint")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if *serveStream {
		mux.HandleFunc("/stream", cameraapp.StreamHandler)
	}
	httpSrv := &http.Server{
		Addr:    strings.TrimSpace(*listen),
		Handler: mux,
	}
	go func() {
		logs.Info("camera_v1 http listening on %s", *listen)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Error("camera_v1 http server failed: %v", err)
		}
	}()

	var nc *nats.Conn
	if strings.TrimSpace(*natsURL) != "" {
		var err error
		nc, err = nats.Connect(strings.TrimSpace(*natsURL), nats.Timeout(2*time.Second))
		if err != nil {
			logs.Warn("camera_v1 nats connect failed; continuing http-only: %v", err)
		}
	}
	if nc != nil {
		defer nc.Close()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	logs.Info("camera_v1 started subject=%s nats=%s listen=%s", *subject, *natsURL, *listen)
	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
			logs.Info("camera_v1 stopping")
			return nil
		case t := <-ticker.C:
			if nc == nil {
				continue
			}
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
	logs.Raw("  run [--nats-url URL] [--subject camera.heartbeat] [--interval 1s] [--listen :19090]")
	logs.Raw("  version")
}
