package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const tailscaleAPIBase = "https://api.tailscale.com"

type provisionOptions struct {
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

type authKeyResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
	Reusable    bool   `json:"reusable"`
	Ephemeral   bool   `json:"ephemeral"`
	Expires     string `json:"expires"`
}

type tailscaleClient struct {
	BaseURL string
	APIKey  string
	Tailnet string
	HTTP    *http.Client
}

func ensureAuthKey(cfg tsnetConfig, envPath string) error {
	if strings.TrimSpace(os.Getenv(cfg.AuthKeyEnv)) != "" {
		return nil
	}

	apiKey := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TS_API_KEY"))
	}
	if apiKey == "" {
		return fmt.Errorf("TS_AUTHKEY missing; set TS_AUTHKEY or TS_API_KEY for bootstrap")
	}
	if strings.TrimSpace(cfg.Tailnet) == "" {
		return errors.New("TS_TAILNET missing; cannot provision TS_AUTHKEY")
	}

	writeEnv := strings.TrimSpace(envPath)
	if writeEnv == "" {
		writeEnv = "env/.env"
	}

	opts := provisionOptions{
		APIToken:      apiKey,
		Tailnet:       cfg.Tailnet,
		Description:   fmt.Sprintf("dialtone-tsnet-%s", cfg.Hostname),
		Tags:          []string{"dialtone", "embedded", "ephemeral"},
		Reusable:      false,
		Ephemeral:     true,
		Preauthorized: true,
		ExpiryHours:   24,
		WriteEnvPath:  writeEnv,
		EnvKeyName:    "TS_AUTHKEY",
	}

	return runProvision(opts)
}

func ensureDialtoneACL(cfg tsnetConfig) error {
	apiKey := strings.TrimSpace(os.Getenv(cfg.APIKeyEnv))
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("TS_API_KEY"))
	}
	if apiKey == "" {
		return nil
	}
	if strings.TrimSpace(cfg.Tailnet) == "" {
		return nil
	}
	hostname := sanitizeHost(cfg.Hostname)
	if hostname == "" {
		hostname = "dialtone-node"
	}
	return ensureMoshACLForHost(apiKey, cfg.Tailnet, hostname)
}

func ensureMoshACLForHost(apiKey, tailnet, hostname string) error {
	client, err := newTailscaleClient(apiKey, tailnet)
	if err != nil {
		return err
	}

	acl, err := client.GetACL()
	if err != nil {
		return fmt.Errorf("fetch ACL failed: %w", err)
	}

	policy, hasWrapper, wrapperKey := unwrapACLPolicy(acl)
	updatedACL, err := mergeMoshACL(policy, []string{hostname})
	if err != nil {
		return err
	}

	updatedTagOwners, err := ensureMeshSSHTag(policy)
	if err != nil {
		return err
	}

	if !updatedACL && !updatedTagOwners {
		return nil
	}

	payload := any(policy)
	if hasWrapper {
		wrapped := map[string]any{}
		for k, v := range acl {
			wrapped[k] = v
		}
		wrapped[wrapperKey] = policy
		payload = wrapped
	}

	if err := client.SetACL(payload); err != nil {
		return fmt.Errorf("set ACL failed: %w", err)
	}
	return nil
}

func runProvision(opts provisionOptions) error {
	if opts.ExpiryHours <= 0 {
		opts.ExpiryHours = 24
	}

	if strings.TrimSpace(opts.WriteEnvPath) == "" {
		opts.WriteEnvPath = "env/.env"
	}
	if strings.TrimSpace(opts.EnvKeyName) == "" {
		opts.EnvKeyName = "TS_AUTHKEY"
	}

	client, err := newTailscaleClient(opts.APIToken, opts.Tailnet)
	if err != nil {
		return err
	}

	resp, err := client.ProvisionKey(opts)
	if err != nil {
		return err
	}

	if err := upsertEnvVar(opts.WriteEnvPath, opts.EnvKeyName, resp.Key); err != nil {
		return err
	}
	return os.Setenv(opts.EnvKeyName, resp.Key)
}

func newTailscaleClient(apiKey, tailnet string) (*tailscaleClient, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("missing Tailscale API key (set TS_API_KEY or pass --api-key)")
	}
	if strings.TrimSpace(tailnet) == "" {
		return nil, errors.New("missing tailnet (set TS_TAILNET or pass --tailnet)")
	}
	return &tailscaleClient{
		BaseURL: tailscaleAPIBase,
		APIKey:  strings.TrimSpace(apiKey),
		Tailnet: strings.TrimSpace(tailnet),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
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

func (c *tailscaleClient) ProvisionKey(opts provisionOptions) (*authKeyResponse, error) {
	var raw map[string]any
	if err := c.do("POST", "/keys", buildCreateKeyRequest(opts), &raw); err != nil {
		return nil, err
	}

	var out authKeyResponse
	if nested, ok := raw["key"]; ok {
		switch typed := nested.(type) {
		case map[string]any:
			if err := mapToStruct(typed, &out); err == nil && out.Key == "" {
				_ = mapToStruct(raw, &out)
			}
		case string:
			out.Key = strings.TrimSpace(typed)
		}
	}
	if strings.TrimSpace(out.Key) == "" {
		if err := mapToStruct(raw, &out); err != nil {
			return nil, errors.New("tailscale API returned empty key response")
		}
	}
	if strings.TrimSpace(out.Key) == "" {
		return nil, errors.New("tailscale API returned empty key")
	}
	return &out, nil
}

func (c *tailscaleClient) GetACL() (map[string]any, error) {
	var acl map[string]any
	if err := c.do("GET", "/acl", nil, &acl); err != nil {
		return nil, err
	}
	if acl == nil {
		acl = map[string]any{}
	}
	return acl, nil
}

func (c *tailscaleClient) SetACL(policy any) error {
	errPost := c.do("POST", "/acl", policy, nil)
	if errPost == nil {
		return nil
	}
	errPut := c.do("PUT", "/policy", policy, nil)
	if errPut == nil {
		return nil
	}
	wrapped := map[string]any{"ACL": policy}
	errWrappedUpper := c.do("POST", "/acl", wrapped, nil)
	if errWrappedUpper == nil {
		return nil
	}
	wrapped = map[string]any{"acl": policy}
	errWrappedLower := c.do("POST", "/acl", wrapped, nil)
	if errWrappedLower == nil {
		return nil
	}
	return fmt.Errorf("tailscale ACL update failed (POST, /policy PUT, wrapped POST variants): post=%v put=%v wrapped=%v/%v", errPost, errPut, errWrappedUpper, errWrappedLower)
}

func mergeMoshACL(policy map[string]any, hostnames []string) (bool, error) {
	acls := policy["acls"]
	aclSlice, ok := acls.([]any)
	if !ok {
		if acls != nil {
			return false, fmt.Errorf("unexpected ACL schema: 'acls' must be array")
		}
		aclSlice = []any{}
	}

	sshRules := policy["ssh"]
	sshSlice, ok := sshRules.([]any)
	if !ok {
		if sshRules != nil {
			return false, fmt.Errorf("unexpected ACL schema: 'ssh' must be array")
		}
		sshSlice = []any{}
	}

	hostnames = dedupeNonEmptyStrings(hostnames)
	if len(hostnames) == 0 {
		return false, errors.New("no hostnames supplied")
	}

	desiredACLs, desiredSSH := buildMoshACLRules(hostnames)
	changed := false
	for _, rule := range desiredACLs {
		if !aclRuleExists(aclSlice, rule) {
			aclSlice = append(aclSlice, rule)
			changed = true
		}
	}
	for _, rule := range desiredSSH {
		if !sshRuleExists(sshSlice, rule) {
			sshSlice = append(sshSlice, rule)
			changed = true
		}
	}
	if changed {
		policy["acls"] = aclSlice
		policy["ssh"] = sshSlice
	}
	return changed, nil
}

func ensureMeshSSHTag(policy map[string]any) (bool, error) {
	const tag = "tag:dialtone"
	ownerSelectors := []string{"autogroup:member"}

	raw, ok := policy["tagOwners"]
	if !ok || raw == nil {
		policy["tagOwners"] = map[string]any{tag: ownerSelectors}
		return true, nil
	}

	tagOwners, ok := raw.(map[string]any)
	if !ok {
		return false, fmt.Errorf("unexpected ACL schema: 'tagOwners' must be object")
	}

	existing, ok := tagOwners[tag]
	if !ok || existing == nil {
		tagOwners[tag] = ownerSelectors
		policy["tagOwners"] = tagOwners
		return true, nil
	}

	existingSlice, okAny := existing.([]any)
	existingList := make([]string, 0, len(existingSlice))
	if okAny {
		for _, item := range existingSlice {
			if selector := strings.TrimSpace(strings.ToLower(anyToString(item))); selector != "" {
				existingList = append(existingList, selector)
			}
		}
	} else if existingStrings, okStringSlice := existing.([]string); okStringSlice {
		for _, selector := range existingStrings {
			if s := strings.TrimSpace(strings.ToLower(selector)); s != "" {
				existingList = append(existingList, s)
			}
		}
	} else {
		return false, fmt.Errorf("unexpected ACL schema: 'tagOwners[%q]' must be array", tag)
	}

	desiredList := dedupeNonEmptyStrings(ownerSelectors)
	changed := false
	for _, item := range desiredList {
		if !stringSliceContains(existingList, item) {
			existingList = append(existingList, item)
			changed = true
		}
	}
	if changed {
		tagOwners[tag] = dedupeNonEmptyStrings(existingList)
		policy["tagOwners"] = tagOwners
	}
	return changed, nil
}

func stringSliceContains(items []string, candidate string) bool {
	for _, item := range items {
		if item == candidate {
			return true
		}
	}
	return false
}

func buildMoshACLRules(hostnames []string) ([]any, []any) {
	if len(hostnames) == 0 {
		return []any{}, []any{}
	}
	return []any{
			map[string]any{
				"action": "accept",
				"src":    []any{"autogroup:member"},
				"dst":    []any{"autogroup:member:60000-61000"},
			},
		}, []any{
			map[string]any{
				"action": "accept",
				"src":    []any{"autogroup:member"},
				"dst":    []any{"tag:dialtone"},
				"users":  []any{"autogroup:nonroot", "root"},
			},
		}
}

func aclRuleExists(rules []any, candidate any) bool {
	candidateRule, ok := candidate.(map[string]any)
	if !ok {
		return false
	}
	normalized := canonicalACLRule(candidateRule)
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		existing := canonicalACLRule(rule)
		if deepEqualStringMap(existing, normalized) {
			return true
		}
	}
	return false
}

func sshRuleExists(rules []any, candidate any) bool {
	candidateRule, ok := candidate.(map[string]any)
	if !ok {
		return false
	}
	normalized := canonicalACLRule(candidateRule)
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		existing := canonicalACLRule(rule)
		if deepEqualStringMap(existing, normalized) {
			return true
		}
	}
	return false
}

func canonicalACLRule(rule map[string]any) map[string]any {
	return map[string]any{
		"action": strings.ToLower(strings.TrimSpace(anyToString(rule["action"]))),
		"src":    normalizeStringList(rule["src"]),
		"dst":    normalizeStringList(rule["dst"]),
		"users":  normalizeStringList(rule["users"]),
	}
}

func normalizeStringList(v any) []any {
	values := make([]string, 0)
	switch typed := v.(type) {
	case []any:
		for _, x := range typed {
			s := strings.TrimSpace(anyToString(x))
			if s != "" {
				values = append(values, strings.ToLower(s))
			}
		}
	case []string:
		for _, s := range typed {
			s = strings.TrimSpace(s)
			if s != "" {
				values = append(values, strings.ToLower(s))
			}
		}
	}
	sort.Strings(values)
	out := make([]any, 0, len(values))
	for _, s := range values {
		out = append(out, s)
	}
	return out
}

func deepEqualStringMap(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for key, av := range a {
		bv, ok := b[key]
		if !ok {
			return false
		}
		if !deepEqualCanonicalValues(av, bv) {
			return false
		}
	}
	return true
}

func deepEqualCanonicalValues(a, b any) bool {
	aa := canonicalSliceOrString(a)
	bb := canonicalSliceOrString(b)
	if len(aa) != len(bb) {
		return false
	}
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func canonicalSliceOrString(v any) []string {
	switch typed := v.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = strings.TrimSpace(strings.ToLower(item))
			if item != "" {
				out = append(out, item)
			}
		}
		sort.Strings(out)
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.TrimSpace(strings.ToLower(anyToString(item)))
			if s != "" {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	case string:
		s := strings.TrimSpace(strings.ToLower(typed))
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func unwrapACLPolicy(raw map[string]any) (map[string]any, bool, string) {
	if raw == nil {
		return map[string]any{}, false, ""
	}
	if aclRaw, ok := raw["acl"]; ok {
		if policy, ok := aclRaw.(map[string]any); ok {
			return policy, true, "acl"
		}
	}
	if aclRaw, ok := raw["ACL"]; ok {
		if policy, ok := aclRaw.(map[string]any); ok {
			return policy, true, "ACL"
		}
	}
	return raw, false, ""
}

func buildCreateKeyRequest(opts provisionOptions) map[string]any {
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

func dedupeNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		s := sanitizeHost(value)
		if s == "" {
			continue
		}
		k := strings.ToLower(s)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, s)
	}
	return out
}

func anyToString(v any) string {
	switch cast := v.(type) {
	case string:
		return cast
	default:
		return ""
	}
}

func mapToStruct(input map[string]any, target any) error {
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
}

func upsertEnvVar(envPath, key, value string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("missing env key")
	}

	if err := ensureParentDir(envPath); err != nil {
		return err
	}

	existing := ""
	if raw, err := os.ReadFile(envPath); err == nil {
		existing = string(raw)
	}

	lines := strings.Split(existing, "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
		lines = append(lines, "")
	}

	prefix := key + "="
	replaced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	joined := strings.Join(lines, "\n")
	if !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	return os.WriteFile(envPath, []byte(joined), 0o644)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return nil
}
