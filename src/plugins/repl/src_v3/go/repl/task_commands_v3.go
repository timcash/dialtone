package repl

import (
	"bufio"
	configv1 "dialtone/dev/plugins/config/src_v1/go"
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

func RunTask(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 task <list|show|log|kill> [args]")
	}
	switch strings.TrimSpace(args[0]) {
	case "list":
		return RunTaskList(args[1:])
	case "show":
		return RunTaskShow(args[1:])
	case "log":
		return RunTaskLog(args[1:])
	case "kill":
		return RunTaskKill(args[1:])
	default:
		return fmt.Errorf("unsupported task command %q (expected list|show|log|kill)", strings.TrimSpace(args[0]))
	}
}

func RunTaskList(args []string) error {
	fs := flag.NewFlagSet("repl-v3-task-list", flag.ContinueOnError)
	count := fs.Int("count", 20, "Number of recent tasks to show")
	state := fs.String("state", "all", "Filter tasks by state: all|running|done")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	host := fs.String("host", "", "Target host name (reserved for mesh host routing)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = strings.TrimSpace(*host)
	if err := ensureTaskQueryLeader(strings.TrimSpace(*natsURL)); err != nil {
		return err
	}

	if items, err := queryTaskRegistry(strings.TrimSpace(*natsURL), *count); err == nil {
		items = filterTaskRegistryItems(items, *state)
		if len(items) == 0 {
			logs.Raw("%s", noTaskListMessage(*state))
			return nil
		}
		logs.Raw("TASK ID                      PID      UPDATED                   STATE    MODE         COMMAND")
		for _, item := range items {
			taskID := strings.TrimSpace(item.TaskID)
			if taskID == "" {
				taskID = "-"
			}
			updated := strings.TrimSpace(item.LastUpdate)
			if updated == "" {
				updated = strings.TrimSpace(item.StartedAt)
			}
			if updated == "" {
				updated = "-"
			}
			cmd := strings.TrimSpace(item.Command)
			if cmd == "" {
				cmd = "-"
			}
			logs.Raw("%-28s %-8d %-24s %-8s %-12s %s", taskID, item.PID, updated, taskStateToken(item.Active), defaultTaskMode(item.Mode), cmd)
		}
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(*state), "running") {
		return fmt.Errorf("task registry is unavailable and running-state fallback is not supported")
	}
	logsDir, err := resolveTaskLogsDir()
	if err != nil {
		return err
	}
	items, err := collectTaskLogs(logsDir)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		logs.Raw("No task logs found in %s", logsDir)
		return nil
	}
	limit := *count
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	logs.Raw("TASK ID                      MODIFIED                  COMMAND")
	for i := 0; i < limit; i++ {
		it := items[i]
		logs.Raw("%-28s %-24s %s", it.TaskID, it.ModTime.Format(time.RFC3339), extractTaskCommand(it.Path))
	}
	return nil
}

func RunTaskShow(args []string) error {
	fs := flag.NewFlagSet("repl-v3-task-show", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "Task identifier")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	host := fs.String("host", "", "Target host name (reserved for mesh host routing)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = strings.TrimSpace(*host)
	if err := ensureTaskQueryLeader(strings.TrimSpace(*natsURL)); err != nil {
		return err
	}

	targetTaskID := strings.TrimSpace(*taskID)
	if targetTaskID == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 task show --task-id <task-id>")
	}
	if item, ok := queryTaskByID(strings.TrimSpace(*natsURL), targetTaskID); ok {
		item.LogPath = preferredTaskLogPath(targetTaskID, item.LogPath)
		printTaskSnapshot(item)
		return nil
	}
	logsDir, err := resolveTaskLogsDir()
	if err != nil {
		return err
	}
	path, err := findTaskLogByTaskID(logsDir, targetTaskID)
	if err != nil {
		return fmt.Errorf("task %s not found", targetTaskID)
	}
	logs.Raw("Task: %s", targetTaskID)
	logs.Raw("Task log: %s", path)
	logs.Raw("Command: %s", extractTaskCommand(path))
	return nil
}

func RunTaskLog(args []string) error {
	fs := flag.NewFlagSet("repl-v3-task-log", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "Task identifier")
	lines := fs.Int("lines", 200, "Max lines to print")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	host := fs.String("host", "", "Target host name (reserved for mesh host routing)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = strings.TrimSpace(*host)
	if err := ensureTaskQueryLeader(strings.TrimSpace(*natsURL)); err != nil {
		return err
	}

	targetTaskID := strings.TrimSpace(*taskID)
	if targetTaskID == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 task log --task-id <task-id> [--lines N]")
	}
	if item, ok := queryTaskByID(strings.TrimSpace(*natsURL), targetTaskID); ok {
		item.LogPath = preferredTaskLogPath(targetTaskID, item.LogPath)
		if strings.TrimSpace(item.LogPath) != "" {
			max := *lines
			if max <= 0 {
				max = 200
			}
			content, err := tailFileLines(strings.TrimSpace(item.LogPath), max)
			if err == nil {
				logs.Raw("Task log: %s", strings.TrimSpace(item.LogPath))
				logs.Raw("%s", content)
				return nil
			}
		}
	}
	logsDir, err := resolveTaskLogsDir()
	if err != nil {
		return err
	}
	path, err := findTaskLogByTaskID(logsDir, targetTaskID)
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
	logs.Raw("Task log: %s", path)
	logs.Raw("%s", content)
	return nil
}

func RunTaskKill(args []string) error {
	fs := flag.NewFlagSet("repl-v3-task-kill", flag.ContinueOnError)
	taskID := fs.String("task-id", "", "Task identifier")
	natsURL := fs.String("nats-url", resolveREPLNATSURL(), "NATS URL for querying the live REPL leader")
	host := fs.String("host", "", "Target host name (reserved for mesh host routing)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = strings.TrimSpace(*host)
	if err := ensureTaskQueryLeader(strings.TrimSpace(*natsURL)); err != nil {
		return err
	}

	targetTaskID := strings.TrimSpace(*taskID)
	if targetTaskID == "" {
		return fmt.Errorf("usage: ./dialtone.sh repl src_v3 task kill --task-id <task-id>")
	}
	item, ok := queryTaskByID(strings.TrimSpace(*natsURL), targetTaskID)
	if !ok {
		return fmt.Errorf("task %s not found", targetTaskID)
	}
	if !item.Active {
		logs.Raw("Task %s is already done.", targetTaskID)
		return nil
	}
	if item.PID <= 0 {
		return fmt.Errorf("task %s has no live pid to stop", targetTaskID)
	}
	logs.Raw("Stopping task %s (pid %d)", targetTaskID, item.PID)
	if err := killManagedProcessFn(item.PID); err != nil {
		return err
	}
	logs.Raw("Stop signal sent to task %s.", targetTaskID)
	return nil
}

func resolveTaskLogsDir() (string, error) {
	return filepath.Join(configv1.DefaultDialtoneHome(), "logs"), nil
}

func ensureTaskQueryLeader(natsURL string) error {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		natsURL = resolveREPLNATSURL()
	}
	return EnsureLeaderRunning(natsURL, defaultRoom)
}

func collectTaskLogs(logsDir string) ([]taskLogMeta, error) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]taskLogMeta, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.TrimSpace(e.Name())
		if !strings.HasPrefix(name, "task-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		taskID, ok := parseTaskIDFromLogName(name)
		if !ok {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, taskLogMeta{
			Path:    filepath.Join(logsDir, name),
			Name:    name,
			TaskID:  taskID,
			ModTime: info.ModTime(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModTime.After(items[j].ModTime) })
	return items, nil
}

func parseTaskIDFromLogName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "task-") || !strings.HasSuffix(name, ".log") {
		return "", false
	}
	taskID := strings.TrimSpace(strings.TrimSuffix(name, ".log"))
	if taskID == "" {
		return "", false
	}
	return taskID, true
}

func findTaskLogByTaskID(logsDir string, taskID string) (string, error) {
	items, err := collectTaskLogs(logsDir)
	if err != nil {
		return "", err
	}
	for _, it := range items {
		if strings.EqualFold(strings.TrimSpace(it.TaskID), strings.TrimSpace(taskID)) {
			return it.Path, nil
		}
	}
	return "", fmt.Errorf("no task log found for %s in %s", taskID, logsDir)
}

func extractTaskCommand(path string) string {
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
	return configv1.ResolveREPLNATSURL()
}

func queryTaskRegistry(natsURL string, count int) ([]taskRegistryItem, error) {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		natsURL = defaultNATSURL
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return nil, err
	}
	defer nc.Close()

	payload, err := json.Marshal(taskRegistryRequest{Count: count})
	if err != nil {
		return nil, err
	}
	msg, err := nc.Request(taskRegistrySubject, payload, 1500*time.Millisecond)
	if err != nil {
		return nil, err
	}
	var items []taskRegistryItem
	if err := json.Unmarshal(msg.Data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func queryTaskByID(natsURL string, taskID string) (taskRegistryItem, bool) {
	items, err := queryTaskRegistry(natsURL, 0)
	if err != nil {
		return taskRegistryItem{}, false
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.TaskID), strings.TrimSpace(taskID)) {
			return item, true
		}
	}
	return taskRegistryItem{}, false
}

func filterTaskRegistryItems(items []taskRegistryItem, state string) []taskRegistryItem {
	filter := strings.TrimSpace(strings.ToLower(state))
	switch filter {
	case "", "all":
		return items
	case "running", "active":
		out := make([]taskRegistryItem, 0, len(items))
		for _, item := range items {
			if item.Active {
				out = append(out, item)
			}
		}
		return out
	case "done":
		out := make([]taskRegistryItem, 0, len(items))
		for _, item := range items {
			if !item.Active {
				out = append(out, item)
			}
		}
		return out
	default:
		return items
	}
}

func noTaskListMessage(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "running", "active":
		return "No running tasks reported by leader."
	case "done":
		return "No completed tasks reported by leader."
	default:
		return "No tasks reported by leader."
	}
}

func taskStateToken(active bool) string {
	if active {
		return "running"
	}
	return "done"
}

func defaultTaskMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return "foreground"
	}
	return mode
}

func printTaskSnapshot(item taskRegistryItem) {
	taskID := strings.TrimSpace(item.TaskID)
	if taskID == "" {
		taskID = "-"
	}
	command := strings.TrimSpace(item.Command)
	if command == "" {
		command = "-"
	}
	mode := defaultTaskMode(item.Mode)
	room := strings.TrimSpace(item.Room)
	if room == "" {
		room = taskRoomName(taskID)
	}
	logPath := preferredTaskLogPath(taskID, item.LogPath)
	updated := strings.TrimSpace(item.LastUpdate)
	if updated == "" {
		updated = strings.TrimSpace(item.StartedAt)
	}
	if updated == "" {
		updated = "-"
	}
	started := strings.TrimSpace(item.StartedAt)
	if started == "" {
		started = "-"
	}
	logs.Raw("Task: %s", taskID)
	logs.Raw("PID: %d", item.PID)
	logs.Raw("State: %s", taskStateToken(item.Active))
	logs.Raw("Mode: %s", mode)
	logs.Raw("Topic: %s", room)
	logs.Raw("Command: %s", command)
	logs.Raw("Started: %s", started)
	logs.Raw("Updated: %s", updated)
	if logPath != "" {
		logs.Raw("Task log: %s", logPath)
	}
	logs.Raw("Exit code: %d", item.ExitCode)
}

func preferredTaskLogPath(taskID string, fallback string) string {
	taskID = strings.TrimSpace(taskID)
	if strings.HasPrefix(taskID, "task-") {
		if logsDir, err := resolveTaskLogsDir(); err == nil {
			path := filepath.Join(logsDir, taskID+".log")
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return strings.TrimSpace(fallback)
}
