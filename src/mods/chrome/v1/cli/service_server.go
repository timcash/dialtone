package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed web/*
var webAssets embed.FS

func runServer(args []string) error {
	opts, err := parseServerOptions(args)
	if err != nil {
		return err
	}

	embeddedSrv, err := maybeStartEmbeddedNATS(opts)
	if err != nil {
		return err
	}
	defer embeddedSrv.Close()

	mgr, err := newChromeServiceManager(opts)
	if err != nil {
		return err
	}
	defer mgr.Close()

	bridge, err := newNATSBridge(opts, mgr)
	if err != nil {
		return err
	}
	defer bridge.Close()

	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if err := mgr.devtoolsHealthy(); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/tabs", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, commandResponse{OK: true, Tabs: mgr.listTabs()})
	})

	addr := fmt.Sprintf("%s:%d", opts.host, opts.port)
	httpServer := &http.Server{Addr: addr, Handler: mux}
	fmt.Printf("chrome embedded server listening on http://%s\n", addr)
	fmt.Printf("chrome nats control listening on %s with prefix %q\n", opts.natsURL, opts.natsPrefix)
	fmt.Printf("subjects: %s.tab.open | %s.tab.close | %s.tab.goto | %s.tab.list\n",
		opts.natsPrefix, opts.natsPrefix, opts.natsPrefix, opts.natsPrefix)

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
