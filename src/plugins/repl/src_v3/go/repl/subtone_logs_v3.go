package repl

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func RunSubtoneList(args []string) error {
	fs := flag.NewFlagSet("repl-v3-subtone-list", flag.ContinueOnError)
	count := fs.Int("count", 20, "Number of recent subtones to show")
	if err := fs.Parse(args); err != nil {
		return err
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pid <= 0 {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 subtone-log --pid <pid> [--lines N]")
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
				return strings.TrimSpace(line[i+5:])
			}
		}
	}
	return "-"
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
