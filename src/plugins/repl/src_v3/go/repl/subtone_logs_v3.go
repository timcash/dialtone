package repl

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

func RunSubtoneList(args []string) error {
	fs := flag.NewFlagSet("repl-v3-subtone-list", flag.ContinueOnError)
	count := fs.Int("count", 20, "Number of recent subtones to show")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if items, err := querySubtoneRegistry(strings.TrimSpace(*natsURL), *count); err == nil {
		if len(items) == 0 {
			logs.Raw("No recent subtones reported by leader.")
			return nil
		}
		logs.Raw("PID      UPDATED                   STATE    COMMAND")
		for _, item := range items {
			state := "done"
			if item.Active {
				state = "active"
			}
			updated := strings.TrimSpace(item.LastUpdate)
			if updated == "" {
				updated = strings.TrimSpace(item.StartedAt)
			}
			cmd := strings.TrimSpace(item.Command)
			if cmd == "" {
				cmd = "-"
			}
			logs.Raw("%-8d %-24s %-8s %s", item.PID, updated, state, cmd)
		}
		return nil
	}
	logsDir, err := resolveSubtoneLogsDir()
	if err != nil {
		return err
	}
	items, err := collectSubtoneLogs(logsDir)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		logs.Raw("No subtone logs found in %s", logsDir)
		return nil
	}
	limit := *count
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	logs.Raw("PID      MODIFIED                  COMMAND")
	for i := 0; i < limit; i++ {
		it := items[i]
		cmd := extractSubtoneCommand(it.Path)
		logs.Raw("%-8d %-24s %s", it.PID, it.ModTime.Format(time.RFC3339), cmd)
	}
	return nil
}

func RunSubtoneLog(args []string) error {
	fs := flag.NewFlagSet("repl-v3-subtone-log", flag.ContinueOnError)
	pid := fs.Int("pid", 0, "Subtone PID")
	lines := fs.Int("lines", 200, "Max lines to print")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid <= 0 {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 subtone-log --pid <pid> [--lines N]")
	}
	if item, ok := querySubtoneByPID(strings.TrimSpace(*natsURL), *pid); ok && strings.TrimSpace(item.LogPath) != "" {
		max := *lines
		if max <= 0 {
			max = 200
		}
		content, err := tailFileLines(strings.TrimSpace(item.LogPath), max)
		if err == nil {
			logs.Raw("Subtone log: %s", strings.TrimSpace(item.LogPath))
			logs.Raw("%s", content)
			return nil
		}
	}
	logsDir, err := resolveSubtoneLogsDir()
	if err != nil {
		return err
	}
	path, err := findSubtoneLogByPID(logsDir, *pid)
	if err != nil {
		return err
	}
	max := *lines
	if max <= 0 {
		max = 200
	}
	content, err := tailFileLines(path, max)
	if err != nil {
		return err
	}
	logs.Raw("Subtone log: %s", path)
	logs.Raw("%s", content)
	return nil
}

func resolveSubtoneLogsDir() (string, error) {
	repoRoot, _, err := resolveRoots()
	if err != nil {
		return "", err
	}
	return filepath.Join(repoRoot, ".dialtone", "logs"), nil
}

func collectSubtoneLogs(logsDir string) ([]subtoneLogMeta, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]subtoneLogMeta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasPrefix(name, "subtone-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		pid, ok := parsePIDFromSubtoneName(name)
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, subtoneLogMeta{
			Path:    filepath.Join(logsDir, name),
			Name:    name,
			PID:     pid,
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModTime.After(items[j].ModTime) })
	return items, nil
}

func parsePIDFromSubtoneName(name string) (int, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(name), "subtone-")
	if trimmed == name {
		return 0, false
	}
	parts := strings.SplitN(trimmed, "-", 2)
	if len(parts) < 1 {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func findSubtoneLogByPID(logsDir string, pid int) (string, error) {
	items, err := collectSubtoneLogs(logsDir)
	if err != nil {
		return "", err
	}
	for _, it := range items {
		if it.PID == pid {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("no subtone log found for pid %d in %s", pid, logsDir)
}

func extractSubtoneCommand(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "-"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.Contains(line, " args=") {
			i := strings.Index(line, "args=")
			if i >= 0 {
				return normalizeLoggedArgs(strings.TrimSpace(line[i+5:]))
			}
		}
	}
	return "-"
}

func normalizeLoggedArgs(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return raw
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
	if body == "" {
		return "-"
	}
	fields := strings.Fields(body)
	if len(fields) == 0 {
		return raw
	}
	args := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if unquoted, err := strconv.Unquote(field); err == nil {
			args = append(args, unquoted)
			continue
		}
		args = append(args, strings.Trim(field, `"`))
	}
	if len(args) == 0 {
		return raw
	}
	return strings.Join(args, " ")
}

func tailFileLines(path string, maxLines int) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}

func resolveREPLNATSURL() string {
	if raw := strings.TrimSpace(os.Getenv("DIALTONE_REPL_NATS_URL")); raw != "" {
		return raw
	}
	return defaultNATSURL
}

func querySubtoneRegistry(natsURL string, count int) ([]subtoneRegistryItem, error) {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		natsURL = defaultNATSURL
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer nc.Close()

	payload, err := json.Marshal(subtoneRegistryRequest{Count: count})
	if err != nil {
		return nil, err
	}
	msg, err := nc.Request(subtoneRegistrySubject, payload, 1500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	var items []subtoneRegistryItem
	if err := json.Unmarshal(msg.Data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func querySubtoneByPID(natsURL string, pid int) (subtoneRegistryItem, bool) {
	items, err := querySubtoneRegistry(natsURL, 0)
	if err != nil {
		return subtoneRegistryItem{}, false
	}
	for _, item := range items {
		if item.PID == pid {
			return item, true
		}
	}
	return subtoneRegistryItem{}, false
}
