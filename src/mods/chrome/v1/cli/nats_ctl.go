package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultTabSelector = "main"
)

type natsControlOptions struct {
	natsURL    string
	natsPrefix string
	tab        string
	url        string
	timeout    time.Duration
}

type controlRequest struct {
	Tab string `json:"tab,omitempty"`
	URL string `json:"url,omitempty"`
}

func parseNATSControlOptions(argv []string) (natsControlOptions, error) {
	fs := flag.NewFlagSet("chrome v1 nats ctl", flag.ContinueOnError)
	natsURL := fs.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	natsPrefix := fs.String("nats-prefix", "chrome.v1", "NATS subject prefix")
	tab := fs.String("tab", defaultTabSelector, "Tab name")
	url := fs.String("url", "", "Target URL")
	timeout := fs.Duration("timeout", 4*time.Second, "Request timeout")
	if err := fs.Parse(argv); err != nil {
		return natsControlOptions{}, err
	}
	if len(fs.Args()) > 0 {
		return natsControlOptions{}, fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}
	opts := natsControlOptions{
		natsURL:    strings.TrimSpace(*natsURL),
		natsPrefix: strings.TrimSpace(*natsPrefix),
		tab:        strings.TrimSpace(*tab),
		url:        strings.TrimSpace(*url),
		timeout:    *timeout,
	}
	if opts.tab == "" {
		opts.tab = defaultTabSelector
	}
	if opts.timeout <= 0 {
		return natsControlOptions{}, fmt.Errorf("--timeout must be > 0")
	}
	return opts, nil
}

func runTabOpen(args []string) error {
	opts, err := parseNATSControlOptions(args)
	if err != nil {
		return err
	}
	resp, err := requestNATSCommand(opts, ".tab.open", controlRequest{Tab: opts.tab, URL: opts.url})
	if err != nil {
		return err
	}
	return printResponse(resp)
}

func runTabClose(args []string) error {
	opts, err := parseNATSControlOptions(args)
	if err != nil {
		return err
	}
	resp, err := requestNATSCommand(opts, ".tab.close", controlRequest{Tab: opts.tab})
	if err != nil {
		return err
	}
	return printResponse(resp)
}

func runTabGoto(args []string) error {
	opts, err := parseNATSControlOptions(args)
	if err != nil {
		return err
	}
	if opts.url == "" {
		return fmt.Errorf("--url is required")
	}
	resp, err := requestNATSCommand(opts, ".tab.goto", controlRequest{Tab: opts.tab, URL: opts.url})
	if err != nil {
		return err
	}
	return printResponse(resp)
}

func runTabList(args []string) error {
	opts, err := parseNATSControlOptions(args)
	if err != nil {
		return err
	}
	resp, err := requestNATSCommand(opts, ".tab.list", controlRequest{})
	if err != nil {
		return err
	}
	return printResponse(resp)
}

func requestNATSCommand(opts natsControlOptions, suffix string, payload controlRequest) (commandResponse, error) {
	nc, err := nats.Connect(opts.natsURL, nats.Name("dialtone-chrome-v1-cli"))
	if err != nil {
		return commandResponse{}, err
	}
	defer nc.Close()

	subject := opts.natsPrefix + suffix
	data, err := json.Marshal(payload)
	if err != nil {
		return commandResponse{}, err
	}
	msg, err := nc.Request(subject, data, opts.timeout)
	if err != nil {
		return commandResponse{}, err
	}
	resp := commandResponse{}
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return commandResponse{}, fmt.Errorf("invalid response: %w", err)
	}
	return resp, nil
}

func printResponse(resp commandResponse) error {
	pretty, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(pretty))
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "request failed"
		}
		return errors.New(resp.Error)
	}
	return nil
}
