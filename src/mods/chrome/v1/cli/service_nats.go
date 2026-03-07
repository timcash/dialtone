package main

import (
	"encoding/json"
	"fmt"
	"net"
	neturl "net/url"
	"strconv"
	"strings"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func maybeStartEmbeddedNATS(opts serverOptions) (*embeddedNATSServer, error) {
	if !opts.embeddedNATS {
		return &embeddedNATSServer{}, nil
	}
	host, port, err := parseNATSHostPort(opts.natsURL)
	if err != nil {
		return nil, err
	}
	srv, err := natsserver.NewServer(&natsserver.Options{
		Host: host,
		Port: port,
	})
	if err != nil {
		return nil, fmt.Errorf("create embedded nats server: %w", err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		srv.Shutdown()
		return nil, fmt.Errorf("embedded nats server did not become ready")
	}
	fmt.Printf("embedded nats server listening on nats://%s:%d\n", host, port)
	return &embeddedNATSServer{server: srv}, nil
}

func (e *embeddedNATSServer) Close() {
	if e == nil || e.server == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	e.server.Shutdown()
}

func parseNATSHostPort(raw string) (string, int, error) {
	u, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", 0, fmt.Errorf("invalid nats url %q: %w", raw, err)
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		host = "127.0.0.1"
	}
	portStr := strings.TrimSpace(u.Port())
	if portStr == "" {
		portStr = "4222"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid nats port in %q", raw)
	}
	if ip := net.ParseIP(host); ip == nil && host != "localhost" {
		return "", 0, fmt.Errorf("nats host %q must be an IP or localhost for embedded mode", host)
	}
	return host, port, nil
}

func newNATSBridge(opts serverOptions, mgr *chromeServiceManager) (*natsBridge, error) {
	nc, err := nats.Connect(opts.natsURL, nats.Name("dialtone-chrome-v1-service"))
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}
	b := &natsBridge{nc: nc, mgr: mgr}
	if err := b.handle(opts.natsPrefix+".tab.open", b.onTabOpen); err != nil {
		b.Close()
		return nil, err
	}
	if err := b.handle(opts.natsPrefix+".tab.close", b.onTabClose); err != nil {
		b.Close()
		return nil, err
	}
	if err := b.handle(opts.natsPrefix+".tab.goto", b.onTabGoto); err != nil {
		b.Close()
		return nil, err
	}
	if err := b.handle(opts.natsPrefix+".tab.list", b.onTabList); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

func (b *natsBridge) handle(subject string, fn func(controlRequest) commandResponse) error {
	sub, err := b.nc.Subscribe(subject, func(msg *nats.Msg) {
		cmd := controlRequest{}
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &cmd); err != nil {
				b.respond(msg, commandResponse{OK: false, Error: fmt.Sprintf("invalid payload: %v", err), Tabs: []tabInfo{}})
				return
			}
		}
		b.respond(msg, fn(cmd))
	})
	if err != nil {
		return err
	}
	b.subs = append(b.subs, sub)
	return nil
}

func (b *natsBridge) onTabOpen(cmd controlRequest) commandResponse {
	fmt.Printf("onTabOpen request: tab=%q url=%q\n", cmd.Tab, cmd.URL)
	info, err := b.mgr.addTab(cmd.Tab, cmd.URL)
	if err != nil {
		fmt.Printf("onTabOpen error: %v\n", err)
		return commandResponse{OK: false, Error: err.Error(), Tabs: []tabInfo{}}
	}
	fmt.Printf("onTabOpen ok: tab=%q target_id=%s\n", info.Name, info.TargetID)
	return commandResponse{OK: true, Tab: info.Name, Tabs: []tabInfo{}}
}

func (b *natsBridge) onTabClose(cmd controlRequest) commandResponse {
	fmt.Printf("onTabClose request: tab=%q\n", cmd.Tab)
	if err := b.mgr.closeTab(cmd.Tab); err != nil {
		fmt.Printf("onTabClose error: %v\n", err)
		return commandResponse{OK: false, Error: err.Error(), Tabs: []tabInfo{}}
	}
	fmt.Printf("onTabClose ok: tab=%q\n", normalizeTabName(cmd.Tab))
	return commandResponse{OK: true, Tab: normalizeTabName(cmd.Tab), Tabs: []tabInfo{}}
}

func (b *natsBridge) onTabGoto(cmd controlRequest) commandResponse {
	fmt.Printf("onTabGoto request: tab=%q url=%q\n", cmd.Tab, cmd.URL)
	if err := b.mgr.gotoTab(cmd.Tab, cmd.URL); err != nil {
		fmt.Printf("onTabGoto error: %v\n", err)
		return commandResponse{OK: false, Error: err.Error(), Tabs: []tabInfo{}}
	}
	fmt.Printf("onTabGoto ok: tab=%q\n", normalizeTabName(cmd.Tab))
	return commandResponse{OK: true, Tab: normalizeTabName(cmd.Tab), Tabs: []tabInfo{}}
}

func (b *natsBridge) onTabList(_ controlRequest) commandResponse {
	return commandResponse{OK: true, Tabs: b.mgr.listTabs()}
}

func (b *natsBridge) respond(msg *nats.Msg, resp commandResponse) {
	if strings.TrimSpace(msg.Reply) == "" {
		return
	}
	data, _ := json.Marshal(resp)
	_ = b.nc.Publish(msg.Reply, data)
}

func (b *natsBridge) Close() {
	if b == nil {
		return
	}
	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	if b.nc != nil {
		b.nc.Close()
	}
}
