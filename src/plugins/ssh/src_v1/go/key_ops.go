package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"golang.org/x/crypto/ssh"
)

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("ssh keygen", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	keyPath := fs.String("key-path", "", "Private key output path")
	force := fs.Bool("force", false, "Overwrite existing key files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	resolved, err := ResolveMeshNode(target)
	if err != nil {
		return err
	}
	path := defaultSSHKeyPathForNode(resolved, *keyPath)
	_, pubPath, err := ensureLocalSSHKeyPair(path, *force)
	if err != nil {
		return err
	}
	if err := upsertMeshNodeAuth(resolved.Name, path, ""); err != nil {
		return err
	}
	logs.Raw("Generated key: %s", path)
	logs.Raw("Public key: %s", pubPath)
	logs.Raw("Updated env/dialtone.json mesh_nodes[%s].ssh_private_key_path", resolved.Name)
	return nil
}

func runKeyInstall(args []string) error {
	fs := flag.NewFlagSet("ssh key-install", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional password for bootstrap auth")
	keyPath := fs.String("key-path", "", "Local private key path to use")
	pubPath := fs.String("pub-key-path", "", "Local public key path (default: <key-path>.pub)")
	generate := fs.Bool("generate", false, "Generate local key if missing")
	forceGenerate := fs.Bool("force-generate", false, "Force regenerate local key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	resolved, err := ResolveMeshNode(target)
	if err != nil {
		return err
	}
	path := defaultSSHKeyPathForNode(resolved, *keyPath)
	if *generate || *forceGenerate {
		if _, _, err := ensureLocalSSHKeyPair(path, *forceGenerate); err != nil {
			return err
		}
	}
	publicPath := strings.TrimSpace(*pubPath)
	if publicPath == "" {
		publicPath = path + ".pub"
	}
	pubBytes, err := os.ReadFile(publicPath)
	if err != nil {
		return fmt.Errorf("read public key %s: %w", publicPath, err)
	}
	pubKey := strings.TrimSpace(string(pubBytes))
	if pubKey == "" {
		return fmt.Errorf("public key is empty: %s", publicPath)
	}

	opts := CommandOptions{
		User:           strings.TrimSpace(*user),
		Port:           strings.TrimSpace(*port),
		Password:       strings.TrimSpace(*pass),
		PrivateKeyPath: path,
	}
	client, nodeInfo, dialHost, dialPort, err := DialMeshNode(resolved.Name, opts)
	if err != nil {
		if strings.TrimSpace(opts.Password) == "" {
			return fmt.Errorf("dial failed before key install; provide --password or configure mesh_nodes[%s].password: %w", resolved.Name, err)
		}
		return err
	}
	defer client.Close()

	if err := installAuthorizedKey(client, pubKey); err != nil {
		return fmt.Errorf("install key on %s failed: %w", resolved.Name, err)
	}
	logs.Raw("Installed public key on %s (%s:%s)", nodeInfo.Name, dialHost, dialPort)

	if err := upsertMeshNodeAuth(resolved.Name, path, ""); err != nil {
		return err
	}
	logs.Raw("Updated env/dialtone.json mesh_nodes[%s].ssh_private_key_path", resolved.Name)
	logs.Raw("Tip: remove mesh_nodes[%s].password after verifying passwordless auth", resolved.Name)
	return nil
}

func runKeySetup(args []string) error {
	fs := flag.NewFlagSet("ssh key-setup", flag.ContinueOnError)
	fs.SetOutput(nil)
	host := fs.String("host", "", "Mesh host name or alias")
	node := fs.String("node", "", "Alias for --host (deprecated)")
	user := fs.String("user", "", "Override remote user")
	port := fs.String("port", "", "Override remote port")
	pass := fs.String("password", "", "Optional password for bootstrap auth")
	keyPath := fs.String("key-path", "", "Local private key path")
	force := fs.Bool("force", false, "Regenerate key before install")
	if err := fs.Parse(args); err != nil {
		return err
	}
	target := strings.TrimSpace(*host)
	if target == "" {
		target = strings.TrimSpace(*node)
	}
	if target == "" {
		return errors.New("--host is required")
	}
	resolved, err := ResolveMeshNode(target)
	if err != nil {
		return err
	}
	path := defaultSSHKeyPathForNode(resolved, *keyPath)
	if _, _, err := ensureLocalSSHKeyPair(path, *force); err != nil {
		return err
	}

	if err := runKeyInstall([]string{
		"--host", resolved.Name,
		"--user", strings.TrimSpace(*user),
		"--port", strings.TrimSpace(*port),
		"--password", strings.TrimSpace(*pass),
		"--key-path", path,
	}); err != nil {
		return err
	}

	out, err := RunNodeCommand(resolved.Name, "printf key-auth-ok", CommandOptions{
		User:           strings.TrimSpace(*user),
		Port:           strings.TrimSpace(*port),
		PrivateKeyPath: path,
	})
	if err != nil {
		return fmt.Errorf("key auth verification failed: %w", err)
	}
	logs.Raw("Verify result: %s", strings.TrimSpace(out))
	return nil
}

func defaultSSHKeyPathForNode(node MeshNode, provided string) string {
	if p := strings.TrimSpace(provided); p != "" {
		return p
	}
	if p := strings.TrimSpace(node.SSHPrivateKeyPath); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone", "keys", fmt.Sprintf("%s_id_ed25519", normalizeTarget(node.Name)))
}

func ensureLocalSSHKeyPair(privatePath string, force bool) (string, string, error) {
	privatePath = strings.TrimSpace(privatePath)
	if privatePath == "" {
		return "", "", fmt.Errorf("key path is required")
	}
	pubPath := privatePath + ".pub"
	if !force {
		if _, err := os.Stat(privatePath); err == nil {
			if _, pubErr := os.Stat(pubPath); pubErr == nil {
				return privatePath, pubPath, nil
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(privatePath), 0o700); err != nil {
		return "", "", fmt.Errorf("mkdir key dir: %w", err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	if err := os.WriteFile(privatePath, privPEM, 0o600); err != nil {
		return "", "", fmt.Errorf("write private key: %w", err)
	}
	pub, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return "", "", fmt.Errorf("encode public key: %w", err)
	}
	if err := os.WriteFile(pubPath, ssh.MarshalAuthorizedKey(pub), 0o644); err != nil {
		return "", "", fmt.Errorf("write public key: %w", err)
	}
	return privatePath, pubPath, nil
}

func installAuthorizedKey(client *ssh.Client, pubKey string) error {
	escaped := shellSingleQuote(pubKey)
	cmd := fmt.Sprintf(
		"umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; grep -qxF %s ~/.ssh/authorized_keys || printf '%%s\\n' %s >> ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys",
		escaped, escaped,
	)
	_, err := RunSSHCommand(client, cmd)
	return err
}

func shellSingleQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'"'"'`) + "'"
}

func upsertMeshNodeAuth(nodeName, keyPath, password string) error {
	configPath, err := meshConfigPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config %s: %w", configPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse json config %s: %w", configPath, err)
	}
	rawNodes, ok := doc["mesh_nodes"]
	if !ok {
		return fmt.Errorf("mesh_nodes not found in %s", configPath)
	}
	nodes, ok := rawNodes.([]any)
	if !ok {
		return fmt.Errorf("mesh_nodes must be an array in %s", configPath)
	}
	updated := false
	for i := range nodes {
		entry, ok := nodes[i].(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if normalizeTarget(name) != normalizeTarget(nodeName) {
			continue
		}
		if strings.TrimSpace(keyPath) != "" {
			entry["ssh_private_key_path"] = keyPath
		}
		if strings.TrimSpace(password) != "" {
			entry["password"] = password
		}
		updated = true
		break
	}
	if !updated {
		return fmt.Errorf("mesh node %q not found in %s", nodeName, configPath)
	}
	doc["mesh_nodes"] = nodes
	pretty, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json config %s: %w", configPath, err)
	}
	pretty = append(pretty, '\n')
	if err := os.WriteFile(configPath, pretty, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}
	resetMeshCache()
	return nil
}
