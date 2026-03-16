package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LogStore struct {
	path string
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Room      string    `json:"room"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Text      string    `json:"text"`
}

func NewLogStore(path string) (*LogStore, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("log path is required")
	}
	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	return &LogStore{path: trimmed}, nil
}

func (s *LogStore) Append(entry LogEntry) error {
	entry.Timestamp = time.Now().UTC()
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write log entry: %w", err)
	}
	return nil
}

func (s *LogStore) ReadAll() ([]LogEntry, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	entries := []LogEntry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("decode log entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file: %w", err)
	}
	return entries, nil
}

func (s *LogStore) Tail(limit int) ([]LogEntry, error) {
	entries, err := s.ReadAll()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit >= len(entries) {
		return entries, nil
	}
	return entries[len(entries)-limit:], nil
}

func (e LogEntry) String() string {
	return fmt.Sprintf("%s [%s] %s/%s %s", e.Timestamp.Format(time.RFC3339), e.Kind, e.Room, e.Name, e.Text)
}

func (e LogEntry) JSON() string {
	data, err := json.Marshal(e)
	if err != nil {
		return "{}"
	}
	return string(data)
}
