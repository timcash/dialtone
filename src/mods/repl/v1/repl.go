package main

import (
	"fmt"
	"strings"
)

type Config struct {
	Name    string
	Room    string
	Prompt  string
	LogPath string
	Once    string
}

type LogsConfig struct {
	LogPath string
	Tail    int
	JSON    bool
}

type Response struct {
	Text string
	Exit bool
}

type Session struct {
	cfg   Config
	store *LogStore
	id    string
}

func NewSession(cfg Config, store *LogStore) *Session {
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = defaultPromptName()
	}
	if strings.TrimSpace(cfg.Room) == "" {
		cfg.Room = "local"
	}
	if strings.TrimSpace(cfg.Prompt) == "" {
		cfg.Prompt = "repl>"
	}
	return &Session{
		cfg:   cfg,
		store: store,
		id:    newSessionID(),
	}
}

func (s *Session) Start() error {
	return s.store.Append(LogEntry{
		SessionID: s.id,
		Room:      s.cfg.Room,
		Name:      s.cfg.Name,
		Kind:      "system",
		Text:      "session started",
	})
}

func (s *Session) HandleLine(line string) (Response, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return Response{}, nil
	}

	if err := s.store.Append(LogEntry{
		SessionID: s.id,
		Room:      s.cfg.Room,
		Name:      s.cfg.Name,
		Kind:      "input",
		Text:      trimmed,
	}); err != nil {
		return Response{}, err
	}

	switch trimmed {
	case ":quit", ":exit", "quit", "exit":
		resp := Response{Text: "bye", Exit: true}
		if err := s.appendOutput(resp.Text); err != nil {
			return Response{}, err
		}
		if err := s.store.Append(LogEntry{
			SessionID: s.id,
			Room:      s.cfg.Room,
			Name:      s.cfg.Name,
			Kind:      "system",
			Text:      "session stopped",
		}); err != nil {
			return Response{}, err
		}
		return resp, nil
	case ":help":
		resp := Response{Text: "commands: :help, :history, :quit"}
		return resp, s.appendOutput(resp.Text)
	case ":history":
		history, err := s.store.Tail(10)
		if err != nil {
			return Response{}, err
		}
		lines := make([]string, 0, len(history))
		for _, entry := range history {
			if entry.Kind != "input" && entry.Kind != "output" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s %s", entry.Kind, entry.Text))
		}
		resp := Response{Text: strings.Join(lines, "\n")}
		return resp, s.appendOutput("history requested")
	default:
		resp := Response{Text: "ok: " + trimmed}
		return resp, s.appendOutput(resp.Text)
	}
}

func (s *Session) appendOutput(text string) error {
	return s.store.Append(LogEntry{
		SessionID: s.id,
		Room:      s.cfg.Room,
		Name:      s.cfg.Name,
		Kind:      "output",
		Text:      text,
	})
}
