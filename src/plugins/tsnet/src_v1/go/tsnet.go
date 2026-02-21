package tsnet

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
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
	case "keys":
		return runKeys(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown tsnet command: %s", args[0])
	}
}

func PrintUsage() {
	fmt.Println("Usage: ./dialtone.sh tsnet <command> [src_v1] [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  config               Show resolved tsnet config")
	fmt.Println("  status               Show tsnet prereq status")
	fmt.Println("  up [--dry-run]       Build/validate tsnet server config")
	fmt.Println("  keys provision [--tailnet N] [--api-key K] [--description D] [--tags t1,t2] [--ephemeral] [--preauthorized] [--reusable] [--expiry-hours N] [--write-env env/.env]")
	fmt.Println("  keys list [--tailnet N] [--api-key K]")
	fmt.Println("  keys revoke <key-id> [--tailnet N] [--api-key K]")
	fmt.Println("  keys usage [--tailnet N] [--api-key K]")
	fmt.Println("  test [src_v1]        Run tsnet plugin self-check")
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
		tailnet = strings.TrimSpace(os.Getenv("TAILSCALE_TAILNET"))
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
	return errors.New("tsnet up without --dry-run is not enabled yet")
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

func printJSON(v any) error {
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
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
