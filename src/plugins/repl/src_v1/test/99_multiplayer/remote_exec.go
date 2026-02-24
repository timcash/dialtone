package multiplayer

import (
	"fmt"
	"os/exec"
	"strings"
)

func runRemoteJoinScript(h hostSpec, natsURL, targetHost, token string) error {
	remote := fmt.Sprintf(
		"set -e; BIN=\"$HOME/.dialtone/bin/dialtone_repl\"; if [ ! -x \"$BIN\" ]; then BIN=\"$HOME/.dialtone/repl/current\"; fi; if [ ! -x \"$BIN\" ]; then echo dialtone_repl-not-found; exit 127; fi; { sleep 1; echo '/go src_v1 version'; sleep 1; echo '@%s echo %s'; sleep 1; echo 'quit'; } | \"$BIN\" join --nats-url %s --name %s --room index",
		shellQuote(targetHost),
		shellQuote(token),
		shellQuote(natsURL),
		shellQuote(h.Name),
	)
	target := fmt.Sprintf("%s@%s", h.User, h.Host)
	cmd := exec.Command("sshpass", "-p", h.Pass, "ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=12", target, remote)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func verifyHostCanDialNATS(h hostSpec, natsURL string) error {
	u, err := natsURLParse(natsURL)
	if err != nil {
		return err
	}
	remote := fmt.Sprintf(
		"python3 -c \"import socket; socket.create_connection(('%s', %s), 2).close()\"",
		strings.ReplaceAll(u.host, "'", "\\'"),
		u.port,
	)
	target := fmt.Sprintf("%s@%s", h.User, h.Host)
	cmd := exec.Command("sshpass", "-p", h.Pass, "ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10", target, remote)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

type parsedNATS struct {
	host string
	port string
}

func natsURLParse(raw string) (parsedNATS, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(raw, "nats://"))
	host := trimmed
	port := "4222"
	if strings.Contains(trimmed, ":") {
		parts := strings.Split(trimmed, ":")
		host = strings.TrimSpace(parts[0])
		port = strings.TrimSpace(parts[len(parts)-1])
	}
	if host == "" {
		return parsedNATS{}, fmt.Errorf("invalid nats url %q", raw)
	}
	if port == "" {
		port = "4222"
	}
	return parsedNATS{host: host, port: port}, nil
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}
