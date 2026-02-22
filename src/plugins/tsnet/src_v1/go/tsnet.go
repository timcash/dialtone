package tsnet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/tsnet"
)

const tailscaleAPIBase = "https://api.tailscale.com"

type Config struct {
	Hostname       string `json:"hostname"`
	StateDir       string `json:"state_dir"`
	AuthKeyPresent bool   `json:"auth_key_present"`
	AuthKeyEnv     string `json:"auth_key_env"`
	Tailnet        string `json:"tailnet"`
	APIKeyPresent  bool   `json:"api_key_present"`
	APIKeyEnv      string `json:"api_key_env"`
}

type Status struct {
	Config            Config `json:"config"`
	TailscaleCLI      string `json:"tailscale_cli"`
	TailscaleCLIFound bool   `json:"tailscale_cli_found"`
}

func Run(args []string) error {
	if len(args) == 0 {
		PrintUsage()
		return nil
	}
	args = stripVersionArg(args)

	switch args[0] {
	case "help", "-h", "--help":
		PrintUsage()
		return nil
	case "config":
		cfg, err := ResolveConfig("", "")
		if err != nil {
			return err
		}
		return printJSON(cfg)
	case "status":
		st, err := GetStatus()
		if err != nil {
			return err
		}
		return printJSON(st)
	case "up":
		return runUp(args[1:])
	case "devices", "computers", "hosts":
		return runDevices(args[1:])
	case "list":
		return runDevices(append([]string{"list"}, args[1:]...))
	case "keys":
		return runKeys(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown tsnet command: %s", args[0])
	}
}

func PrintUsage() {
	logs.Raw("Usage: ./dialtone.sh tsnet src_v1 <command> [args]")
	logs.Raw("")
	logs.Raw("Commands:")
	logs.Raw("  config                               Show resolved tsnet config")
	logs.Raw("  status                               Show tsnet prereq status")
	logs.Raw("  up [--dry-run]                       Start embedded tsnet (ephemeral); auto-provision TS_AUTHKEY when needed")
	logs.Raw("  devices list [--tailnet N] [--api-key K] [--format report|json] [--all]")
	logs.Raw("  devices prune --name-contains S [--tailnet N] [--api-key K] [--dry-run] [--yes]")
	logs.Raw("  computers list [--tailnet N] [--api-key K] [--format report|json] [--all]")
	logs.Raw("  list [--tailnet N] [--api-key K] [--format report|json] [--all]  Alias for devices list")
	logs.Raw("  keys provision [--tailnet N] [--api-key K] [--description D] [--tags t1,t2] [--ephemeral] [--preauthorized] [--reusable] [--expiry-hours N] [--write-env env/.env]")
	logs.Raw("  keys list [--tailnet N] [--api-key K]")
	logs.Raw("  keys revoke <key-id> [--tailnet N] [--api-key K]")
	logs.Raw("  keys usage [--tailnet N] [--api-key K]")
	logs.Raw("  test                                 Run tsnet plugin self-check")
}

func ResolveConfig(hostname, stateDir string) (Config, error) {
	if strings.TrimSpace(hostname) == "" {
		hostname = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
	}
	if strings.TrimSpace(hostname) == "" {
		hostname = "dialtone-node"
	}
	hostname = NormalizeHostname(hostname)
	if hostname == "" {
		return Config{}, errors.New("resolved empty hostname")
	}

	if strings.TrimSpace(stateDir) == "" {
		stateDir = strings.TrimSpace(os.Getenv("DIALTONE_TSNET_STATE_DIR"))
	}
	if strings.TrimSpace(stateDir) == "" {
		stateDir = filepath.Join(".dialtone", "tsnet")
	}

	authVar := "TS_AUTHKEY"
	authVal := strings.TrimSpace(os.Getenv("TS_AUTHKEY"))
	if authVal == "" {
		authVal = strings.TrimSpace(os.Getenv("TAILSCALE_AUTHKEY"))
		if authVal != "" {
			authVar = "TAILSCALE_AUTHKEY"
		}
	}

	apiVar := "TS_API_KEY"
	apiVal := strings.TrimSpace(os.Getenv("TS_API_KEY"))
	if apiVal == "" {
		apiVal = strings.TrimSpace(os.Getenv("TAILSCALE_API_KEY"))
		if apiVal != "" {
			apiVar = "TAILSCALE_API_KEY"
		}
	}
	tailnet := strings.TrimSpace(os.Getenv("TS_TAILNET"))
	if tailnet == "" {
		detected, err := DetectTailnetFromLocalStatus()
		if err == nil {
			tailnet = detected
			logs.Debug("tsnet auto-detected tailnet from local tailscale status: %s", tailnet)
		}
	}
	if tailnet == "" {
		tailnet = "shad-artichoke.ts.net"
	}

	return Config{
		Hostname:       hostname,
		StateDir:       stateDir,
		AuthKeyPresent: authVal != "",
		AuthKeyEnv:     authVar,
		Tailnet:        tailnet,
		APIKeyPresent:  apiVal != "",
		APIKeyEnv:      apiVar,
	}, nil
}

func BuildServer(cfg Config) *tsnet.Server {
	authKey := ""
	if cfg.AuthKeyPresent {
		authKey = os.Getenv(cfg.AuthKeyEnv)
	}
	return &tsnet.Server{
		Hostname: cfg.Hostname,
		Dir:      cfg.StateDir,
		AuthKey:  authKey,
		Logf: func(format string, args ...any) {
			logs.Debug("[TSNET] "+format, args...)
		},
	}
}

func GetStatus() (Status, error) {
	cfg, err := ResolveConfig("", "")
	if err != nil {
		return Status{}, err
	}
	path, lookErr := exec.LookPath("tailscale")
	return Status{
		Config:            cfg,
		TailscaleCLI:      path,
		TailscaleCLIFound: lookErr == nil,
	}, nil
}

func NormalizeHostname(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9\-]+`)
	s = re.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func DetectTailnetFromLocalStatus() (string, error) {
	if tailnet, err := detectTailnetFromLocalAPI(); err == nil && strings.TrimSpace(tailnet) != "" {
		return tailnet, nil
	}

	path, err := exec.LookPath("tailscale")
	if err != nil {
		return "", err
	}
	out, err := exec.Command(path, "status", "--json").Output()
	if err != nil {
		return "", err
	}
	tailnet := ParseTailnetFromStatusJSON(out)
	if tailnet == "" {
		return "", errors.New("tailnet not found in tailscale status")
	}
	return tailnet, nil
}

func detectTailnetFromLocalAPI() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	st, err := (&local.Client{}).Status(ctx)
	if err != nil {
		return "", err
	}
	if st == nil {
		return "", errors.New("local tailscale status is nil")
	}
	if st.CurrentTailnet != nil {
		if suffix := sanitizeTailnet(st.CurrentTailnet.MagicDNSSuffix); suffix != "" {
			return suffix, nil
		}
		if name := sanitizeTailnet(st.CurrentTailnet.Name); name != "" {
			return name, nil
		}
	}
	if suffix := sanitizeTailnet(st.MagicDNSSuffix); suffix != "" {
		return suffix, nil
	}
	if st.Self != nil {
		if inferred := inferTailnetFromDNSName(st.Self.DNSName); inferred != "" {
			return inferred, nil
		}
	}
	return "", errors.New("tailnet not found in local tailscale status")
}

func ParseTailnetFromStatusJSON(raw []byte) string {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	if current, ok := doc["CurrentTailnet"].(map[string]any); ok {
		if suffix := sanitizeTailnet(anyToString(current["MagicDNSSuffix"])); suffix != "" {
			return suffix
		}
		if name := sanitizeTailnet(anyToString(current["Name"])); name != "" {
			return name
		}
	}
	if suffix := sanitizeTailnet(anyToString(doc["MagicDNSSuffix"])); suffix != "" {
		return suffix
	}
	if self, ok := doc["Self"].(map[string]any); ok {
		if dnsName := sanitizeTailnet(anyToString(self["DNSName"])); dnsName != "" {
			if inferred := inferTailnetFromDNSName(dnsName); inferred != "" {
				return inferred
			}
		}
	}
	return ""
}

func inferTailnetFromDNSName(dnsName string) string {
	dnsName = sanitizeTailnet(dnsName)
	if dnsName == "" {
		return ""
	}
	parts := strings.Split(dnsName, ".")
	if len(parts) < 2 {
		return ""
	}
	return sanitizeTailnet(strings.Join(parts[1:], "."))
}

func sanitizeTailnet(s string) string {
	return strings.Trim(strings.TrimSpace(s), ".")
}

func anyToString(v any) string {
	s, _ := v.(string)
	return s
}

func runUp(args []string) error {
	dryRun := false
	for _, arg := range args {
		if arg == "--dry-run" {
			dryRun = true
		}
	}

	cfg, err := ResolveConfig("", "")
	if err != nil {
		return err
	}
	srv := BuildServer(cfg)
	if dryRun {
		logs.Info("tsnet dry-run: hostname=%s dir=%s auth_key_env=%s present=%t",
			srv.Hostname, srv.Dir, cfg.AuthKeyEnv, cfg.AuthKeyPresent)
		return nil
	}

	if err := ensureAuthKeyForEmbedded(&cfg); err != nil {
		return err
	}
	srv = BuildServer(cfg)
	srv.Ephemeral = true

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st, err := srv.Up(ctx)
	if err != nil {
		return fmt.Errorf("embedded tsnet up failed: %w", err)
	}

	ip4, ip6 := srv.TailscaleIPs()
	logs.Info("embedded tsnet started: backend=%s self=%s ips=%s,%s tailnet=%s ephemeral=true",
		chooseNonEmpty(st.BackendState, "-"),
		chooseNonEmpty(hostnameFromDNSName(statusSelfDNSName(st)), "-"),
		chooseNonEmpty(ip4.String(), "-"),
		chooseNonEmpty(ip6.String(), "-"),
		chooseNonEmpty(cfg.Tailnet, "-"),
	)
	logs.Info("embedded tsnet running; press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	logs.Info("stopping embedded tsnet")
	return srv.Close()
}

type ProvisionOptions struct {
	APIToken      string
	Tailnet       string
	Description   string
	Tags          []string
	Reusable      bool
	Ephemeral     bool
	Preauthorized bool
	ExpiryHours   int
	WriteEnvPath  string
	EnvKeyName    string
}

type AuthKey struct {
	ID            string   `json:"id"`
	Key           string   `json:"key"`
	Description   string   `json:"description"`
	Reusable      bool     `json:"reusable"`
	Ephemeral     bool     `json:"ephemeral"`
	Preauthorized bool     `json:"preauthorized"`
	Tags          []string `json:"tags"`
	Created       string   `json:"created"`
	Expires       string   `json:"expires"`
	CreatedBy     string   `json:"createdBy"`
}

type Device struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Hostname  string   `json:"hostname"`
	User      string   `json:"user"`
	Created   string   `json:"created"`
	LastSeen  string   `json:"lastSeen"`
	Online    bool     `json:"online"`
	Addresses []string `json:"addresses"`
	Tags      []string `json:"tags"`
}

type KeyUsage struct {
	KeyID       string   `json:"key_id"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Matches     []string `json:"matches"`
	Confidence  string   `json:"confidence"`
	Note        string   `json:"note"`
}

type tailscaleClient struct {
	BaseURL string
	APIKey  string
	Tailnet string
	HTTP    *http.Client
}

func runKeys(args []string) error {
	args = stripAllVersionArgs(args)
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh tsnet keys <provision|list|revoke|usage> ...")
	}

	sub := args[0]
	rest := args[1:]
	switch sub {
	case "provision":
		return runKeysProvision(rest)
	case "list":
		return runKeysList(rest)
	case "revoke":
		return runKeysRevoke(rest)
	case "usage":
		return runKeysUsage(rest)
	default:
		return fmt.Errorf("unknown keys subcommand: %s", sub)
	}
}

func runDevices(args []string) error {
	args = stripAllVersionArgs(args)
	if len(args) == 0 {
		args = []string{"list"}
	}

	sub := args[0]
	switch sub {
	case "list":
		return runDevicesList(args[1:])
	case "prune":
		return runDevicesPrune(args[1:])
	default:
		return fmt.Errorf("unknown devices subcommand: %s", sub)
	}
}

func runDevicesList(args []string) error {
	format := "report"
	activeOnly := true
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--format":
			if i+1 < len(args) {
				format = strings.ToLower(strings.TrimSpace(args[i+1]))
				i++
			}
		case "--all":
			activeOnly = false
		case "--active-only":
			activeOnly = true
		}
	}

	client, err := clientFromArgs(args)
	devices := []Device{}
	if err == nil {
		devices, err = client.ListDevices()
		if err != nil {
			return err
		}
	} else if isControlAPICredsError(err) {
		// Fallback to local daemon status when control-plane credentials are
		// not configured.
		devices, err = listDevicesFromLocalStatus()
		if err != nil {
			// Last fallback for WSL/no-host-daemon scenarios:
			// bring up an embedded ephemeral tsnet and inspect its view.
			devices, err = listDevicesFromEmbeddedTSNet()
			if err != nil {
				return err
			}
		}
	} else {
		return err
	}
	devices = filterActiveDevices(devices, activeOnly)

	switch format {
	case "json":
		return printJSON(devices)
	case "table", "report":
		return printDevicesReport(devices, activeOnly)
	default:
		return fmt.Errorf("unsupported --format %q (expected report or json)", format)
	}
}

func runDevicesPrune(args []string) error {
	nameContains := ""
	dryRun := true
	yes := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name-contains":
			if i+1 < len(args) {
				nameContains = strings.TrimSpace(args[i+1])
				i++
			}
		case "--dry-run":
			dryRun = true
		case "--yes":
			yes = true
			dryRun = false
		}
	}
	if nameContains == "" {
		nameContains = "drone-1"
	}

	client, err := clientFromArgs(args)
	if err != nil {
		return err
	}
	devices, err := client.ListDevices()
	if err != nil {
		return err
	}

	matches := make([]Device, 0)
	for _, d := range devices {
		if containsFold(d.Name, nameContains) || containsFold(d.Hostname, nameContains) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		logs.Info("No devices matched substring %q", nameContains)
		return nil
	}

	for _, d := range matches {
		logs.Info("Matched device id=%s name=%s hostname=%s user=%s",
			chooseNonEmpty(d.ID, "-"), chooseNonEmpty(d.Name, "-"), chooseNonEmpty(d.Hostname, "-"), chooseNonEmpty(d.User, "-"))
	}

	if dryRun || !yes {
		logs.Warn("Dry run only: %d device(s) matched. Re-run with --yes to delete.", len(matches))
		return nil
	}

	deleted := 0
	for _, d := range matches {
		if err := client.DeleteDevice(d.ID); err != nil {
			return fmt.Errorf("delete device %s failed: %w", d.ID, err)
		}
		deleted++
		logs.Info("Deleted device id=%s name=%s", d.ID, chooseNonEmpty(d.Name, d.Hostname, d.ID))
	}
	logs.Info("Prune complete: deleted %d device(s) matching %q", deleted, nameContains)
	return nil
}

func ensureAuthKeyForEmbedded(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	if cfg.AuthKeyPresent {
		return nil
	}

	apiKey := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TS_API_KEY"))
	}
	if apiKey == "" {
		return errors.New("missing TS_AUTHKEY and cannot auto-provision: set TS_API_KEY (or pass --api-key to keys provision)")
	}
	if strings.TrimSpace(cfg.Tailnet) == "" {
		return errors.New("missing tailnet for auto-provision (set TS_TAILNET)")
	}

	opts := ProvisionOptions{
		APIToken:      apiKey,
		Tailnet:       cfg.Tailnet,
		Description:   fmt.Sprintf("dialtone-tsnet-embedded-%s", cfg.Hostname),
		Tags:          []string{"dialtone", "embedded", "ephemeral"},
		Reusable:      false,
		Ephemeral:     true,
		Preauthorized: true,
		ExpiryHours:   24,
		WriteEnvPath:  "env/.env",
		EnvKeyName:    "TS_AUTHKEY",
	}
	key, err := ProvisionAuthKey(opts)
	if err != nil {
		return fmt.Errorf("auto-provision auth key failed: %w", err)
	}
	if err := UpsertEnvVar(opts.WriteEnvPath, opts.EnvKeyName, key.Key); err != nil {
		return fmt.Errorf("write %s failed: %w", opts.WriteEnvPath, err)
	}
	_ = os.Setenv(opts.EnvKeyName, key.Key)

	cfg.AuthKeyPresent = true
	cfg.AuthKeyEnv = opts.EnvKeyName
	cfg.APIKeyPresent = true
	logs.Info("Auto-provisioned ephemeral TS_AUTHKEY for embedded tsnet and saved to %s", opts.WriteEnvPath)
	return nil
}

func isControlAPICredsError(err error) bool {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "missing tailscale api key") || strings.Contains(msg, "missing tailnet")
}

func listDevicesFromLocalStatus() ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	st, err := (&local.Client{}).Status(ctx)
	if err != nil {
		return nil, err
	}
	if st == nil {
		return nil, errors.New("local tailscale status is nil")
	}
	return devicesFromIPNStatus(st), nil
}

func listDevicesFromEmbeddedTSNet() ([]Device, error) {
	cfg, err := ResolveConfig("", "")
	if err != nil {
		return nil, err
	}
	if err := ensureAuthKeyForEmbedded(&cfg); err != nil {
		return nil, err
	}
	srv := BuildServer(cfg)
	srv.Ephemeral = true
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	st, err := srv.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("embedded tsnet fallback failed: %w", err)
	}
	if st == nil {
		return nil, errors.New("embedded tsnet returned nil status")
	}
	return devicesFromIPNStatus(st), nil
}

func devicesFromIPNStatus(st *ipnstate.Status) []Device {
	devices := make([]Device, 0, len(st.Peer)+1)
	seen := map[string]struct{}{}
	add := func(ps *ipnstate.PeerStatus) {
		if ps == nil {
			return
		}
		d := deviceFromPeerStatus(ps, st.User)
		key := chooseNonEmpty(d.ID, d.Name, d.Hostname)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		devices = append(devices, d)
	}

	add(st.Self)
	for _, ps := range st.Peer {
		add(ps)
	}
	sort.Slice(devices, func(i, j int) bool {
		return chooseNonEmpty(devices[i].Name, devices[i].Hostname, devices[i].ID) <
			chooseNonEmpty(devices[j].Name, devices[j].Hostname, devices[j].ID)
	})
	return devices
}

func deviceFromPeerStatus(ps *ipnstate.PeerStatus, users map[tailcfg.UserID]tailcfg.UserProfile) Device {
	addrs := make([]string, 0, len(ps.TailscaleIPs))
	for _, ip := range ps.TailscaleIPs {
		addrs = append(addrs, ip.String())
	}

	user := ""
	if profile, ok := users[ps.UserID]; ok {
		user = chooseNonEmpty(profile.LoginName, profile.DisplayName)
	}
	if user == "" {
		user = fmt.Sprintf("%d", ps.UserID)
	}

	tags := []string{}
	if ps.Tags != nil {
		tags = ps.Tags.AppendTo(tags)
	}

	hostname := chooseNonEmpty(ps.HostName, hostnameFromDNSName(ps.DNSName))
	return Device{
		ID:        string(ps.ID),
		Name:      chooseNonEmpty(hostnameFromDNSName(ps.DNSName), ps.HostName, string(ps.ID)),
		Hostname:  hostname,
		User:      user,
		Created:   formatTime(ps.Created),
		LastSeen:  formatTime(ps.LastSeen),
		Online:    ps.Online,
		Addresses: addrs,
		Tags:      tags,
	}
}

func hostnameFromDNSName(dnsName string) string {
	dnsName = strings.Trim(strings.TrimSpace(dnsName), ".")
	if dnsName == "" {
		return ""
	}
	parts := strings.Split(dnsName, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func statusSelfDNSName(st *ipnstate.Status) string {
	if st == nil || st.Self == nil {
		return ""
	}
	return st.Self.DNSName
}

func runKeysProvision(args []string) error {
	cfg, err := ResolveConfig("", "")
	if err != nil {
		return err
	}
	opts := ProvisionOptions{
		Tailnet:       cfg.Tailnet,
		APIToken:      strings.TrimSpace(os.Getenv(cfg.APIKeyEnv)),
		Description:   fmt.Sprintf("dialtone-tsnet-%s", cfg.Hostname),
		Tags:          nil,
		Reusable:      false,
		Ephemeral:     true,
		Preauthorized: true,
		ExpiryHours:   24,
		WriteEnvPath:  "env/.env",
		EnvKeyName:    "TS_AUTHKEY",
	}
	if opts.APIToken == "" {
		opts.APIToken = strings.TrimSpace(os.Getenv("TS_API_KEY"))
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tailnet":
			if i+1 < len(args) {
				opts.Tailnet = strings.TrimSpace(args[i+1])
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				opts.APIToken = strings.TrimSpace(args[i+1])
				i++
			}
		case "--description":
			if i+1 < len(args) {
				opts.Description = strings.TrimSpace(args[i+1])
				i++
			}
		case "--tags":
			if i+1 < len(args) {
				opts.Tags = parseCSV(args[i+1])
				i++
			}
		case "--ephemeral":
			opts.Ephemeral = true
		case "--no-ephemeral":
			opts.Ephemeral = false
		case "--reusable":
			opts.Reusable = true
		case "--preauthorized":
			opts.Preauthorized = true
		case "--expiry-hours":
			if i+1 < len(args) {
				n, parseErr := strconv.Atoi(args[i+1])
				if parseErr == nil && n > 0 {
					opts.ExpiryHours = n
				}
				i++
			}
		case "--write-env":
			if i+1 < len(args) {
				opts.WriteEnvPath = strings.TrimSpace(args[i+1])
				i++
			}
		case "--env-key":
			if i+1 < len(args) {
				opts.EnvKeyName = strings.TrimSpace(args[i+1])
				i++
			}
		}
	}

	key, err := ProvisionAuthKey(opts)
	if err != nil {
		return err
	}

	if err := UpsertEnvVar(opts.WriteEnvPath, opts.EnvKeyName, key.Key); err != nil {
		return err
	}

	logs.Info("Provisioned tsnet auth key id=%s reusable=%t ephemeral=%t preauthorized=%t expires=%s",
		key.ID, key.Reusable, key.Ephemeral, key.Preauthorized, key.Expires)
	logs.Info("Wrote %s to %s", opts.EnvKeyName, opts.WriteEnvPath)
	return nil
}

func runKeysList(args []string) error {
	client, err := clientFromArgs(args)
	if err != nil {
		return err
	}
	keys, err := client.ListKeys()
	if err != nil {
		return err
	}
	return printJSON(keys)
}

func runKeysRevoke(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh tsnet keys revoke <key-id> [--tailnet N] [--api-key K]")
	}
	keyID := args[0]
	client, err := clientFromArgs(args[1:])
	if err != nil {
		return err
	}
	if err := client.RevokeKey(keyID); err != nil {
		return err
	}
	logs.Info("Revoked key %s", keyID)
	return nil
}

func runKeysUsage(args []string) error {
	client, err := clientFromArgs(args)
	if err != nil {
		return err
	}
	keys, err := client.ListKeys()
	if err != nil {
		return err
	}
	devices, err := client.ListDevices()
	if err != nil {
		return err
	}
	usage := InferKeyUsage(keys, devices)
	return printJSON(usage)
}

func ProvisionAuthKey(opts ProvisionOptions) (*AuthKey, error) {
	client, err := newClient(opts.APIToken, opts.Tailnet)
	if err != nil {
		return nil, err
	}
	if opts.ExpiryHours <= 0 {
		opts.ExpiryHours = 24
	}

	reqBody := BuildCreateKeyRequest(opts)
	var resp struct {
		Key AuthKey `json:"key"`
	}
	if err := client.do("POST", "/keys", reqBody, &resp); err != nil {
		return nil, err
	}
	if strings.TrimSpace(resp.Key.Key) == "" {
		return nil, errors.New("tailscale API returned empty key")
	}
	return &resp.Key, nil
}

func BuildCreateKeyRequest(opts ProvisionOptions) map[string]any {
	tags := make([]string, 0, len(opts.Tags))
	for _, t := range opts.Tags {
		t = strings.TrimSpace(t)
		if t != "" {
			if !strings.HasPrefix(t, "tag:") {
				t = "tag:" + t
			}
			tags = append(tags, t)
		}
	}
	return map[string]any{
		"capabilities": map[string]any{
			"devices": map[string]any{
				"create": map[string]any{
					"reusable":      opts.Reusable,
					"ephemeral":     opts.Ephemeral,
					"preauthorized": opts.Preauthorized,
					"tags":          tags,
				},
			},
		},
		"expirySeconds": opts.ExpiryHours * 3600,
		"description":   opts.Description,
	}
}

func InferKeyUsage(keys []AuthKey, devices []Device) []KeyUsage {
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })

	usages := make([]KeyUsage, 0, len(keys))
	for _, key := range keys {
		u := KeyUsage{
			KeyID:       key.ID,
			Description: key.Description,
			Tags:        append([]string{}, key.Tags...),
			Matches:     []string{},
			Confidence:  "low",
			Note:        "Tailscale API does not provide direct key->device attribution; matches are inferred by tags/description/name overlap.",
		}
		descSlug := NormalizeHostname(key.Description)
		for _, d := range devices {
			score := 0
			if descSlug != "" {
				if strings.Contains(NormalizeHostname(d.Hostname), descSlug) || strings.Contains(NormalizeHostname(d.Name), descSlug) {
					score += 2
				}
			}
			if hasIntersect(key.Tags, d.Tags) {
				score += 2
			}
			if strings.TrimSpace(key.CreatedBy) != "" && strings.Contains(strings.ToLower(strings.TrimSpace(d.User)), strings.ToLower(strings.TrimSpace(key.CreatedBy))) {
				score++
			}
			if score > 0 {
				u.Matches = append(u.Matches, fmt.Sprintf("%s (hostname=%s, user=%s, score=%d)", chooseNonEmpty(d.Name, d.ID), d.Hostname, d.User, score))
				if score >= 4 {
					u.Confidence = "high"
				} else if score >= 2 && u.Confidence != "high" {
					u.Confidence = "medium"
				}
			}
		}
		if len(u.Matches) == 0 {
			u.Confidence = "none"
		}
		usages = append(usages, u)
	}
	return usages
}

func clientFromArgs(args []string) (*tailscaleClient, error) {
	cfg, err := ResolveConfig("", "")
	if err != nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	tailnet := cfg.Tailnet

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--tailnet":
			if i+1 < len(args) {
				tailnet = strings.TrimSpace(args[i+1])
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				apiKey = strings.TrimSpace(args[i+1])
				i++
			}
		}
	}
	return newClient(apiKey, tailnet)
}

func newClient(apiKey, tailnet string) (*tailscaleClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("missing Tailscale API key (set TS_API_KEY or pass --api-key)")
	}
	if strings.TrimSpace(tailnet) == "" {
		return nil, errors.New("missing tailnet (set TS_TAILNET or pass --tailnet)")
	}
	return &tailscaleClient{
		BaseURL: tailscaleAPIBase,
		APIKey:  apiKey,
		Tailnet: tailnet,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *tailscaleClient) ListKeys() ([]AuthKey, error) {
	var resp struct {
		Keys []AuthKey `json:"keys"`
	}
	if err := c.do("GET", "/keys", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Keys, nil
}

func (c *tailscaleClient) RevokeKey(keyID string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return errors.New("missing key id")
	}
	return c.do("DELETE", "/keys/"+url.PathEscape(keyID), nil, nil)
}

func (c *tailscaleClient) ListDevices() ([]Device, error) {
	var resp struct {
		Devices []Device `json:"devices"`
	}
	if err := c.do("GET", "/devices", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Devices, nil
}

func (c *tailscaleClient) DeleteDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errors.New("missing device id")
	}
	return c.do("DELETE", "device/"+url.PathEscape(deviceID), nil, nil)
}

func (c *tailscaleClient) do(method, path string, body any, out any) error {
	path = strings.TrimPrefix(path, "/")
	endpoint := fmt.Sprintf("%s/api/v2/tailnet/%s/%s", strings.TrimSuffix(c.BaseURL, "/"), url.PathEscape(c.Tailnet), path)

	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewBuffer(raw)
	}

	req, err := http.NewRequest(method, endpoint, payload)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.APIKey, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBytes))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("tailscale api %s %s failed: %s", method, endpoint, msg)
	}
	if out != nil && len(respBytes) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return err
		}
	}
	return nil
}

func UpsertEnvVar(envPath, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("missing env key")
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil && filepath.Dir(envPath) != "." {
		return err
	}

	existing := ""
	if raw, err := os.ReadFile(envPath); err == nil {
		existing = string(raw)
	}
	lines := []string{}
	if existing != "" {
		lines = strings.Split(existing, "\n")
	}

	prefix := key + "="
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			replaced = true
		}
	}
	if !replaced {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	out := strings.Join(lines, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return os.WriteFile(envPath, []byte(out), 0o644)
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasIntersect(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := map[string]struct{}{}
	for _, x := range a {
		x = strings.TrimSpace(strings.ToLower(x))
		if x != "" {
			set[x] = struct{}{}
		}
	}
	for _, y := range b {
		y = strings.TrimSpace(strings.ToLower(y))
		if _, ok := set[y]; ok {
			return true
		}
	}
	return false
}

func chooseNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func containsFold(s, part string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	part = strings.TrimSpace(strings.ToLower(part))
	if s == "" || part == "" {
		return false
	}
	return strings.Contains(s, part)
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ",")
}

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	logs.Raw("%s", string(out))
	return nil
}

func printDevicesReport(devices []Device, activeOnly bool) error {
	if len(devices) == 0 {
		if activeOnly {
			logs.Raw("No active devices found.")
		} else {
			logs.Raw("No devices found.")
		}
		return nil
	}
	mode := "active-only"
	if !activeOnly {
		mode = "all"
	}
	logs.Raw("Tailnet Device Report (%s, count=%d)", mode, len(devices))
	logs.Raw("STATUS\tNAME\tHOSTNAME\tUSER\tTAILSCALE_IPS\tLAST_SEEN\tTAGS")
	for _, d := range devices {
		status := "offline"
		if isDeviceActive(d) {
			status = "active"
		}
		logs.Raw("%s\t%s\t%s\t%s\t%s\t%s\t%s",
			status,
			chooseNonEmpty(d.Name, d.ID),
			chooseNonEmpty(d.Hostname, "-"),
			chooseNonEmpty(d.User, "-"),
			joinOrDash(d.Addresses),
			chooseNonEmpty(d.LastSeen, "-"),
			joinOrDash(d.Tags),
		)
	}
	return nil
}

func filterActiveDevices(devices []Device, activeOnly bool) []Device {
	if !activeOnly {
		return devices
	}
	out := make([]Device, 0, len(devices))
	for _, d := range devices {
		if isDeviceActive(d) {
			out = append(out, d)
		}
	}
	return out
}

func isDeviceActive(d Device) bool {
	if d.Online {
		return true
	}
	lastSeen := strings.TrimSpace(d.LastSeen)
	if lastSeen == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, lastSeen)
	if err != nil {
		return false
	}
	// API payloads are not always explicit about online state; recent
	// lastSeen is treated as active for report defaults.
	return time.Since(t) <= 10*time.Minute
}

func stripVersionArg(args []string) []string {
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return append([]string{args[0]}, args[2:]...)
	}
	return args
}

func stripAllVersionArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if strings.HasPrefix(a, "src_v") {
			continue
		}
		out = append(out, a)
	}
	return out
}
