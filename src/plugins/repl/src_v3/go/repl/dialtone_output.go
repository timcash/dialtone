package repl

import (
	"fmt"
	"io"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

// DialtoneIndexFrame is the one canonical way to create a user-facing
// dialtone> line on the shared REPL index topic. All non-bootstrap REPL
// output should flow through this helper so it stays obvious how to publish
// top-level operator output. The shell bootstrap path in dialtone.sh is the
// only intentional exception.
func DialtoneIndexFrame(frame BusFrame) BusFrame {
	frame.Type = frameTypeLine
	frame.Scope = "index"
	frame.Kind = strings.TrimSpace(frame.Kind)
	if frame.Kind == "" {
		frame.Kind = "status"
	}
	frame.Message = strings.TrimSpace(frame.Message)
	if strings.TrimSpace(frame.Room) != "" {
		frame.Room = sanitizeRoom(frame.Room)
	}
	return frame
}

func EmitDialtoneIndexFrame(emit func(BusFrame), frame BusFrame) {
	if emit == nil {
		return
	}
	frame = DialtoneIndexFrame(frame)
	if frame.Message == "" {
		return
	}
	emit(frame)
}

func EmitDialtoneIndexLine(emit func(BusFrame), kind, message string) {
	EmitDialtoneIndexFrame(emit, BusFrame{Kind: kind, Message: message})
}

func PublishDialtoneIndexFrame(publishRoom func(string, BusFrame), room string, frame BusFrame) {
	if publishRoom == nil {
		return
	}
	frame = DialtoneIndexFrame(frame)
	if frame.Message == "" {
		return
	}
	publishRoom(sanitizeRoom(room), frame)
}

func PublishDialtoneIndexLine(publishRoom func(string, BusFrame), room, kind, message string) {
	PublishDialtoneIndexFrame(publishRoom, room, BusFrame{Kind: kind, Message: message})
}

func FormatDialtoneLine(prefix, message string) string {
	return logs.FormatDialtoneMessage(prefix, message)
}

func WriteDialtoneLine(w io.Writer, prefix, message string) {
	fmt.Fprintf(w, "%s\n", FormatDialtoneLine(prefix, message))
}

func WriteDialtoneSystemLine(w io.Writer, message string) {
	WriteDialtoneLine(w, "dialtone", message)
}
