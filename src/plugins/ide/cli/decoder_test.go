package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestStreamChatLogs(t *testing.T) {
	// 1. Create a temp file
	f, err := os.CreateTemp("", "chatlogs_test_*.pb")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	
	// 2. Start streamer in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out bytes.Buffer
	
	done := make(chan bool)
	go func() {
		StreamChatLogs(ctx, f.Name(), &out)
		done <- true
	}()

	// 3. Write some data to file
	// Structure: [Varint Len] [Body]
	// Body: Field 1 (Bytes) -> Field 11 (Bytes, Repeated) -> Message
	// Message: Field 1 (Bytes) = "Hello World", Field 2 (Varint) = 1 (PROMPT/USER)
	
	msgInner := []byte{}
	msgInner = protowire.AppendTag(msgInner, 1, protowire.BytesType)
	msgInner = protowire.AppendString(msgInner, "Hello World")
	msgInner = protowire.AppendTag(msgInner, 2, protowire.VarintType)
	msgInner = protowire.AppendVarint(msgInner, 1) // Role USER
	
	// Wrap in Field 11 (Message Item)
	// We can have multiple items, but let's do one for now
	// Wait, Conversation message usually has repeated Field 11.
	// So we construct the 'Conversation' object which contains Field 11.
	
	convBody := []byte{}
	convBody = protowire.AppendTag(convBody, 11, protowire.BytesType)
	convBody = protowire.AppendBytes(convBody, msgInner)
	
	// Wrap in Field 1 (Conversation Root)
	rootBody := []byte{}
	rootBody = protowire.AppendTag(rootBody, 1, protowire.BytesType)
	rootBody = protowire.AppendBytes(rootBody, convBody)
	
	// Write length prefix + rootBody
	// Length is varint of len(rootBody)
	finalPayload := protowire.AppendVarint([]byte{}, uint64(len(rootBody)))
	finalPayload = append(finalPayload, rootBody...)
	
	if _, err := f.Write(finalPayload); err != nil {
		t.Fatal(err)
	}
	// Sync to ensure reader sees it? `f.Sync()` might be good but `Write` usually enough for `Open`'d reader.
	// OS buffering might delay visibility to other fd.
	f.Sync()
	
	// 4. Wait a bit for streamer to pick it up
	time.Sleep(2 * time.Second)
	
	cancel()
	<-done
	f.Close()
	
	// 5. Verify output
	output := out.String()
	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected 'Hello World' in output, got: %q", output)
	}
	if !strings.Contains(output, "[CHAT]") {
		t.Errorf("Expected '[CHAT]' in output, got: %q", output)
	}
	if !strings.Contains(output, "USER") {
		t.Errorf("Expected 'USER' role in output, got: %q", output)
	}
}
