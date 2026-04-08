package repl

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"dialtone/dev/plugins/proc/src_v1/go/proc"
	"github.com/nats-io/nats.go"
	gopsprocess "github.com/shirou/gopsutil/v3/process"
)

const taskKVBucketName = "repl_task_v3"

type taskKVRecord struct {
	TaskID    string   `json:"task_id,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Topic     string   `json:"topic,omitempty"`
	LogPath   string   `json:"log_path,omitempty"`
	Host      string   `json:"host,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	State     string   `json:"state,omitempty"`
	PID       int      `json:"pid,omitempty"`
	ExitCode  *int     `json:"exit_code,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
	StartedAt string   `json:"started_at,omitempty"`
	LastOKAt  string   `json:"last_ok_at,omitempty"`
	Service   string   `json:"service,omitempty"`
	WorkerLog string   `json:"worker_log,omitempty"`
}

type taskKVStore struct {
	kv nats.KeyValue
}

func newTaskKVStore(nc *nats.Conn) (*taskKVStore, error) {
	if nc == nil {
		return nil, fmt.Errorf("nil nats connection")
	}
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := ensureTaskKVBucket(js)
	if err != nil {
		return nil, err
	}
	return &taskKVStore{kv: kv}, nil
}

func ensureTaskKVBucket(js nats.JetStreamContext) (nats.KeyValue, error) {
	if js == nil {
		return nil, fmt.Errorf("nil jetstream context")
	}
	kv, err := js.KeyValue(taskKVBucketName)
	if err == nil {
		return kv, nil
	}
	if !errors.Is(err, nats.ErrBucketNotFound) {
		return nil, err
	}
	return js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:      taskKVBucketName,
		Description: "REPL task state",
		History:     1,
	})
}

func taskKVKey(taskID string) string {
	return strings.TrimSpace(taskID)
}

func taskKVHost(host string) string {
	host = normalizePromptName(host)
	if host == "" {
		host = normalizePromptName(DefaultPromptName())
	}
	if host == "" {
		host = "local"
	}
	return host
}

func taskRecordState(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "queued", "running", "done":
		return strings.TrimSpace(strings.ToLower(state))
	default:
		return "queued"
	}
}

func taskStateActive(state string) bool {
	switch taskRecordState(state) {
	case "queued", "running":
		return true
	default:
		return false
	}
}

func (s *taskKVStore) PutQueued(taskID string, args []string, topic, logPath, host, mode, service string) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("task kv store is not available")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := taskKVRecord{
		TaskID:    strings.TrimSpace(taskID),
		Command:   strings.TrimSpace(strings.Join(args, " ")),
		Args:      append([]string(nil), args...),
		Topic:     strings.TrimSpace(topic),
		LogPath:   strings.TrimSpace(logPath),
		Host:      taskKVHost(host),
		Mode:      defaultTaskMode(mode),
		State:     "queued",
		CreatedAt: now,
		UpdatedAt: now,
		Service:   strings.TrimSpace(service),
	}
	return s.put(record)
}

func (s *taskKVStore) MarkRunning(taskID string, ev proc.TaskWorkerEvent) error {
	record, err := s.Get(taskID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.State = "running"
	record.UpdatedAt = now
	record.LastOKAt = now
	if ev.PID > 0 {
		record.PID = ev.PID
	}
	if !ev.StartedAt.IsZero() {
		record.StartedAt = ev.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if strings.TrimSpace(ev.LogPath) != "" {
		record.WorkerLog = strings.TrimSpace(ev.LogPath)
	}
	if len(ev.Args) > 0 {
		record.Args = append([]string(nil), ev.Args...)
		record.Command = strings.TrimSpace(strings.Join(ev.Args, " "))
	}
	return s.put(record)
}

func (s *taskKVStore) MarkHeartbeat(taskID string) error {
	record, err := s.Get(taskID)
	if err != nil {
		return err
	}
	if taskRecordState(record.State) == "done" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.UpdatedAt = now
	record.LastOKAt = now
	if taskRecordState(record.State) == "queued" {
		record.State = "running"
	}
	return s.put(record)
}

func (s *taskKVStore) MarkExited(taskID string, pid int, exitCode int) error {
	record, err := s.Get(taskID)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record.State = "done"
	record.UpdatedAt = now
	if pid > 0 {
		record.PID = pid
	}
	record.LastOKAt = now
	code := exitCode
	record.ExitCode = &code
	return s.put(record)
}

func (s *taskKVStore) Get(taskID string) (taskKVRecord, error) {
	var record taskKVRecord
	if s == nil || s.kv == nil {
		return record, fmt.Errorf("task kv store is not available")
	}
	entry, err := s.kv.Get(taskKVKey(taskID))
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return record, err
	}
	record.TaskID = strings.TrimSpace(record.TaskID)
	return record, nil
}

func (s *taskKVStore) put(record taskKVRecord) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("task kv store is not available")
	}
	record.TaskID = strings.TrimSpace(record.TaskID)
	record.Topic = strings.TrimSpace(record.Topic)
	record.Host = taskKVHost(record.Host)
	record.Mode = defaultTaskMode(record.Mode)
	record.State = taskRecordState(record.State)
	if record.TaskID == "" {
		return fmt.Errorf("task id is required")
	}
	if record.Topic == "" {
		record.Topic = taskRoomName(record.TaskID)
	}
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if record.UpdatedAt == "" {
		record.UpdatedAt = record.CreatedAt
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = s.kv.Put(taskKVKey(record.TaskID), payload)
	return err
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

	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.KeyValue(taskKVBucketName)
	if err != nil {
		if errors.Is(err, nats.ErrBucketNotFound) {
			return nil, nil
		}
		return nil, err
	}
	keys, err := kv.Keys()
	if err != nil {
		if errors.Is(err, nats.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, err
	}
	records := make([]taskKVRecord, 0, len(keys))
	for _, key := range keys {
		entry, getErr := kv.Get(strings.TrimSpace(key))
		if getErr != nil {
			if errors.Is(getErr, nats.ErrKeyNotFound) {
				continue
			}
			return nil, getErr
		}
		record := taskKVRecord{}
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return parseTaskKVTimestamp(records[i].UpdatedAt).After(parseTaskKVTimestamp(records[j].UpdatedAt))
	})
	if count > 0 && len(records) > count {
		records = records[:count]
	}
	out := make([]taskRegistryItem, 0, len(records))
	for _, record := range records {
		out = append(out, taskKVRecordToRegistryItem(record))
	}
	return out, nil
}

func (s *taskKVStore) ReconcileLocalRuntime(host string, managed []proc.ManagedProcessSnapshot) error {
	return s.reconcileLocalRuntime(host, managed, time.Now().UTC(), liveManagedProcessStartTime)
}

func (s *taskKVStore) reconcileLocalRuntime(host string, managed []proc.ManagedProcessSnapshot, now time.Time, inspect func(int) (time.Time, bool)) error {
	if s == nil || s.kv == nil {
		return fmt.Errorf("task kv store is not available")
	}
	host = taskKVHost(host)
	keys, err := s.kv.Keys()
	if err != nil {
		if errors.Is(err, nats.ErrNoKeysFound) {
			return nil
		}
		return err
	}
	managedByPID := map[int]proc.ManagedProcessSnapshot{}
	for _, snap := range managed {
		if snap.PID > 0 {
			managedByPID[snap.PID] = snap
		}
	}
	for _, key := range keys {
		entry, getErr := s.kv.Get(strings.TrimSpace(key))
		if getErr != nil {
			if errors.Is(getErr, nats.ErrKeyNotFound) {
				continue
			}
			return getErr
		}
		record := taskKVRecord{}
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			return err
		}
		next, changed := reconcileTaskKVRecord(record, host, managedByPID, now, inspect)
		if !changed {
			continue
		}
		if err := s.put(next); err != nil {
			return err
		}
	}
	return nil
}

func queryTaskByID(natsURL string, taskID string) (taskRegistryItem, bool) {
	natsURL = strings.TrimSpace(natsURL)
	if natsURL == "" {
		natsURL = defaultNATSURL
	}
	nc, err := nats.Connect(natsURL, nats.Timeout(1200*time.Millisecond))
	if err != nil {
		return taskRegistryItem{}, false
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return taskRegistryItem{}, false
	}
	kv, err := js.KeyValue(taskKVBucketName)
	if err != nil {
		return taskRegistryItem{}, false
	}
	entry, err := kv.Get(taskKVKey(taskID))
	if err != nil {
		return taskRegistryItem{}, false
	}
	record := taskKVRecord{}
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return taskRegistryItem{}, false
	}
	return taskKVRecordToRegistryItem(record), true
}

func taskKVRecordToRegistryItem(record taskKVRecord) taskRegistryItem {
	item := taskRegistryItem{
		TaskID:    strings.TrimSpace(record.TaskID),
		Host:      taskKVHost(record.Host),
		Room:      strings.TrimSpace(record.Topic),
		Topic:     strings.TrimSpace(record.Topic),
		Command:   strings.TrimSpace(record.Command),
		Args:      append([]string(nil), record.Args...),
		Mode:      defaultTaskMode(record.Mode),
		LogPath:   strings.TrimSpace(record.LogPath),
		StartedAt: strings.TrimSpace(record.StartedAt),
		CreatedAt: strings.TrimSpace(record.CreatedAt),
		UpdatedAt: strings.TrimSpace(record.UpdatedAt),
		LastUpdate: func() string {
			if strings.TrimSpace(record.UpdatedAt) != "" {
				return strings.TrimSpace(record.UpdatedAt)
			}
			return strings.TrimSpace(record.CreatedAt)
		}(),
		Active: taskStateActive(record.State),
		State:  taskRecordState(record.State),
	}
	if item.TaskID == "" {
		item.TaskID = "-"
	}
	if item.Room == "" {
		item.Room = taskRoomName(item.TaskID)
	}
	if item.Topic == "" {
		item.Topic = item.Room
	}
	if record.PID > 0 {
		item.PID = record.PID
	}
	if record.ExitCode != nil {
		item.ExitCode = *record.ExitCode
	}
	startedAt := parseTaskKVTimestamp(item.StartedAt)
	if startedAt.IsZero() {
		startedAt = parseTaskKVTimestamp(item.CreatedAt)
	}
	if !startedAt.IsZero() {
		item.StartedAt = startedAt.UTC().Format(time.RFC3339)
		uptime := time.Since(startedAt).Round(time.Second)
		if uptime < 0 {
			uptime = 0
		}
		item.StartedAgo = uptime.String()
	}
	if updatedAt := parseTaskKVTimestamp(item.LastUpdate); !updatedAt.IsZero() {
		item.LastUpdate = updatedAt.UTC().Format(time.RFC3339)
		item.UpdatedAt = item.LastUpdate
	}
	return item
}

func parseTaskKVTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			return ts
		}
	}
	return time.Time{}
}

func reconcileTaskKVRecord(record taskKVRecord, host string, managedByPID map[int]proc.ManagedProcessSnapshot, now time.Time, inspect func(int) (time.Time, bool)) (taskKVRecord, bool) {
	if taskKVHost(record.Host) != taskKVHost(host) {
		return record, false
	}
	state := taskRecordState(record.State)
	switch state {
	case "queued":
		if record.PID > 0 && taskKVRecordMatchesLiveProcess(record, managedByPID, inspect) {
			ts := now.Format(time.RFC3339Nano)
			if record.State != "running" || record.UpdatedAt != ts || record.LastOKAt != ts {
				record.State = "running"
				record.UpdatedAt = ts
				record.LastOKAt = ts
				return record, true
			}
		}
	case "running":
		if taskKVRecordMatchesLiveProcess(record, managedByPID, inspect) {
			ts := now.Format(time.RFC3339Nano)
			if record.UpdatedAt != ts || record.LastOKAt != ts {
				record.UpdatedAt = ts
				record.LastOKAt = ts
				return record, true
			}
			return record, false
		}
		ts := now.Format(time.RFC3339Nano)
		record.State = "done"
		record.UpdatedAt = ts
		if record.ExitCode == nil {
			code := -1
			record.ExitCode = &code
		}
		return record, true
	}
	return record, false
}

func taskKVRecordMatchesLiveProcess(record taskKVRecord, managedByPID map[int]proc.ManagedProcessSnapshot, inspect func(int) (time.Time, bool)) bool {
	if record.PID <= 0 {
		return false
	}
	if _, ok := managedByPID[record.PID]; ok {
		return true
	}
	startedAt := parseTaskKVTimestamp(record.StartedAt)
	if startedAt.IsZero() {
		startedAt = parseTaskKVTimestamp(record.CreatedAt)
	}
	liveStartedAt, ok := inspect(record.PID)
	if !ok {
		return false
	}
	if startedAt.IsZero() || liveStartedAt.IsZero() {
		return true
	}
	delta := liveStartedAt.Sub(startedAt)
	if delta < 0 {
		delta = -delta
	}
	return delta <= 30*time.Second
}

func liveManagedProcessStartTime(pid int) (time.Time, bool) {
	if pid <= 0 {
		return time.Time{}, false
	}
	procHandle, err := gopsprocess.NewProcess(int32(pid))
	if err != nil {
		return time.Time{}, false
	}
	running, err := procHandle.IsRunning()
	if err != nil || !running {
		return time.Time{}, false
	}
	createdMS, err := procHandle.CreateTime()
	if err != nil || createdMS <= 0 {
		return time.Time{}, true
	}
	return time.UnixMilli(createdMS).UTC(), true
}
