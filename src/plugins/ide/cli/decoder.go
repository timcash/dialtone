package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"dialtone/dev/core/logger"
	"google.golang.org/protobuf/encoding/protowire"
)

// StreamChatLogs tails the specified protobuf file and writes to out
func StreamChatLogs(ctx context.Context, pbPath string, out io.Writer) {
	if out == nil {
		out = os.Stdout
	}

	if pbPath == "" {
		pbPath = findRecentConversationProto()
	}
	if pbPath == "" {
		logger.LogFatal("No active conversation found in ~/.gemini/antigravity/conversations/")
	}

	logger.LogInfo("Tailing conversation: %s", pbPath)

	f, err := os.Open(pbPath)
	if err != nil {
		logger.LogFatal("Failed to open pb file: %v", err)
	}
	defer f.Close()

	// Parse from the beginning
	offset := int64(0)
	headerBuf := make([]byte, 10)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 1. Read the length prefix
		_, err := f.Seek(offset, 0)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		n, err := f.Read(headerBuf)
		if err != nil && err != io.EOF {
			time.Sleep(1 * time.Second)
			continue
		}
		if n == 0 {
			// EOF, wait
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Attempt to consume a varint
		v, nLen := protowire.ConsumeVarint(headerBuf[:n])
		if nLen < 0 {
			// Incomplete varint?
			time.Sleep(500 * time.Millisecond)
			continue
		}

		msgLen := int(v)
		totalNeeded := nLen + msgLen

		// 2. Read the full message
		fullBuf := make([]byte, totalNeeded)
		_, err = f.Seek(offset, 0)
		if err != nil {
			continue
		}

		nRead, err := io.ReadFull(f, fullBuf)
		if err != nil {
			// Not enough data yet
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if nRead == totalNeeded {
			// We have the full message. Decode the BODY (skipping length prefix?)
			// Wait, the file format is [Varint Length] [Message Body]
			// I read [Varint + Body] into fullBuf.
			// I should decode fullBuf[nLen:]

			decodeAndPrintMessage(fullBuf[nLen:], out)
			offset += int64(totalNeeded)
		}
	}
}

func decodeAndPrintMessage(data []byte, out io.Writer) {
	// Walk fields looking for content
	// We look for Field 1 -> Field 11 (Repeated) -> Item -> Field 1 (Text)

	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		if typ != protowire.BytesType {
			m := protowire.ConsumeFieldValue(num, typ, data)
			if m < 0 {
				break
			}
			data = data[m:]
			continue
		}

		val, m := protowire.ConsumeBytes(data)
		if m < 0 {
			break
		}
		data = data[m:]

		if num == 1 {
			// Found Field 1 (Conversation/Context?)
			decodeLevel2(val, out)
		}
	}
}

func decodeLevel2(data []byte, out io.Writer) {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		if typ != protowire.BytesType {
			m := protowire.ConsumeFieldValue(num, typ, data)
			if m < 0 {
				break
			}
			data = data[m:]
			continue
		}

		val, m := protowire.ConsumeBytes(data)
		if m < 0 {
			break
		}
		data = data[m:]

		if num == 11 {
			// Found Field 11 (Message Item)
			decodeMessage(val, out)
		}
	}
}

func decodeMessage(data []byte, out io.Writer) {
	var content string
	var role string = "UNKNOWN"

	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]

		if typ == protowire.BytesType {
			val, m := protowire.ConsumeBytes(data)
			if m < 0 {
				break
			}
			data = data[m:]

			if num == 1 {
				// Content
				if utf8.Valid(val) {
					content = string(val)
				}
			}
			// Heuristic for role
			if utf8.Valid(val) && len(val) < 20 {
				str := string(val)
				if strings.Contains(strings.ToLower(str), "user") {
					role = "USER"
				} else if strings.Contains(strings.ToLower(str), "model") || strings.Contains(strings.ToLower(str), "assistant") {
					role = "MODEL"
				}
			}
		} else if typ == protowire.VarintType {
			v, m := protowire.ConsumeVarint(data)
			if m < 0 {
				break
			}
			data = data[m:]

			if num == 2 || num == 3 {
				if v == 1 {
					role = "USER"
				}
				if v == 2 {
					role = "MODEL"
				}
			}
		} else {
			m := protowire.ConsumeFieldValue(num, typ, data)
			if m < 0 {
				break
			}
			data = data[m:]
		}
	}

	if content != "" {
		colorGreen := "\033[32m"
		colorReset := "\033[0m"

		prefix := fmt.Sprintf("%s[CHAT]%s", colorGreen, colorReset)
		timestamp := time.Now().Format("15:04:05")

		if role == "UNKNOWN" {
			role = "MSG"
		}

		fmt.Fprintf(out, "%s %s (%s): %s\n", prefix, timestamp, role, content)
	}
}

func findRecentConversationProto() string {
	home, _ := os.UserHomeDir()
	dir := home + "/.gemini/antigravity/conversations"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var bestFile string
	var bestTime int64

	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".pb") {
			info, _ := e.Info()
			if info.ModTime().Unix() > bestTime {
				bestTime = info.ModTime().Unix()
				bestFile = filepath.Join(dir, e.Name())
			}
		}
	}
	return bestFile
}
