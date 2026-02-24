package multiplayer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	repl "dialtone/dev/plugins/repl/src_v1/go/repl"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
	"github.com/nats-io/nats.go"
)

type observedFrame struct {
	Subject string
	Frame   repl.BusFrame
}

func Register(r *testv1.Registry) {
	r.Add(testv1.Step{
		Name:    "repl-multiplayer-three-users-room-switch-over-nats",
		Timeout: 90 * time.Second,
		RunWithContext: func(ctx *testv1.StepContext) (testv1.StepRunResult, error) {
			var stage atomic.Value
			stage.Store("init")
			stopHeartbeat := make(chan struct{})
			go func() {
				t := time.NewTicker(5 * time.Second)
				defer t.Stop()
				for {
					select {
					case <-t.C:
						s := stage.Load()
						if v, ok := s.(string); ok {
							ctx.Infof("multiplayer stage=%s", v)
						}
					case <-stopHeartbeat:
						return
					}
				}
			}()
			defer close(stopHeartbeat)

			paths, err := repl.ResolvePaths("")
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			dialtoneBin := filepath.Join(paths.Runtime.RepoRoot, "dialtone.sh")
			natsURL := strings.TrimSpace(ctx.NATSURL())
			if natsURL == "" {
				natsURL = "nats://127.0.0.1:4222"
			}
			nc := ctx.NATSConn()
			if nc == nil {
				return testv1.StepRunResult{}, fmt.Errorf("NATS connection unavailable in test context")
			}

			var (
				obsMu sync.Mutex
				obs   []observedFrame
			)
			sub, err := nc.Subscribe("repl.>", func(msg *nats.Msg) {
				var frame repl.BusFrame
				if json.Unmarshal(msg.Data, &frame) != nil {
					return
				}
				obsMu.Lock()
				obs = append(obs, observedFrame{Subject: msg.Subject, Frame: frame})
				obsMu.Unlock()
			})
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			defer sub.Unsubscribe()
			_ = nc.Flush()

			leaderCtx, cancelLeader := context.WithCancel(context.Background())
			defer cancelLeader()
			leaderCmd := exec.CommandContext(
				leaderCtx,
				dialtoneBin,
				"repl", "src_v1", "leader",
				"--embedded-nats=false",
				"--nats-url", natsURL,
				"--room", "index",
				"--hostname", "DIALTONE-SERVER",
			)
			leaderCmd.Dir = paths.Runtime.RepoRoot
			var leaderOut bytes.Buffer
			leaderCmd.Stdout = &leaderOut
			leaderCmd.Stderr = &leaderOut
			if err := leaderCmd.Start(); err != nil {
				return testv1.StepRunResult{}, err
			}
			defer func() {
				cancelLeader()
				if leaderCmd.Process != nil {
					_ = leaderCmd.Process.Kill()
				}
			}()

			if err := waitFor(10*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "server"
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("leader did not publish ready frame: %w\nframes:\n%s\nleader:\n%s", err, snapshotFrames(&obsMu, &obs, 40), leaderOut.String())
			}
			ctx.Infof("leader ready")

			stage.Store("seed-joins")
			users := []string{"user-1", "user-2", "user-3"}
			for _, u := range users {
				if err := publish(nc, "repl.room.index", repl.BusFrame{Type: "join", From: u, Room: "index"}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := waitFor(5*time.Second, func() bool {
				return countFrames(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "join"
				}) >= 3
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("expected three join events: %w\n%s", err, snapshotFrames(&obsMu, &obs, 60))
			}
			stage.Store("command-who-versions")
			if err := publish(nc, "repl.cmd", repl.BusFrame{Type: "command", From: "user-1", Room: "index", Version: "v1.2.3", Message: "/repl src_v1 who"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(8*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" &&
						of.Frame.Type == "line" &&
						strings.Contains(of.Frame.Message, "- user-1 (room=index version=v1.2.3)")
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("who output with version not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 80))
			}
			if err := publish(nc, "repl.cmd", repl.BusFrame{Type: "command", From: "user-1", Room: "index", Version: "v1.2.3", Message: "/repl src_v1 versions"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(8*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" &&
						of.Frame.Type == "line" &&
						strings.Contains(of.Frame.Message, "- user-1 version=v1.2.3 room=index")
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("versions output not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 80))
			}

			stage.Store("chat-index")
			if err := publish(nc, "repl.room.index", repl.BusFrame{Type: "chat", From: "user-1", Room: "index", Message: "hello from user-1"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(4*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "chat" && of.Frame.From == "user-1"
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("chat frame not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 60))
			}

			stage.Store("command-room-switch")
			if err := publish(nc, "repl.cmd", repl.BusFrame{Type: "command", From: "user-2", Room: "index", Message: "/repl src_v1 join ops"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(8*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "control" && of.Frame.Target == "user-2" && of.Frame.Command == "join_room" && of.Frame.Room == "ops"
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("room control frame not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 80))
			}
			stage.Store("apply-room-switch")
			if err := publish(nc, "repl.room.ops", repl.BusFrame{Type: "join", From: "user-2", Room: "ops"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(4*time.Second, func() bool {
				leftSeen := hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "left" && of.Frame.From == "user-2"
				})
				joinSeen := hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.ops" && of.Frame.Type == "join" && of.Frame.From == "user-2"
				})
				return leftSeen && joinSeen
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("room transition sequence not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 80))
			}

			stage.Store("chat-ops")
			if err := publish(nc, "repl.room.ops", repl.BusFrame{Type: "chat", From: "user-2", Room: "ops", Message: "hello from ops"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(4*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.ops" && of.Frame.Type == "chat" && of.Frame.From == "user-2"
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("ops chat not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 90))
			}

			stage.Store("command-go-version")
			if err := publish(nc, "repl.cmd", repl.BusFrame{Type: "command", From: "user-3", Room: "index", Message: "/go src_v1 version"}); err != nil {
				return testv1.StepRunResult{}, err
			}
			if err := waitFor(25*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool {
					return of.Subject == "repl.room.index" && of.Frame.Type == "line" && strings.Contains(of.Frame.Message, "Subtone for go src_v1 exited with code 0")
				})
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("subtone completion not observed: %w\n%s", err, snapshotFrames(&obsMu, &obs, 120))
			}

			stage.Store("left-events")
			for _, u := range users {
				room := "index"
				if u == "user-2" {
					room = "ops"
				}
				if err := publish(nc, "repl.room."+room, repl.BusFrame{Type: "left", From: u, Room: room}); err != nil {
					return testv1.StepRunResult{}, err
				}
			}
			if err := waitFor(4*time.Second, func() bool {
				return hasFrame(&obsMu, &obs, func(of observedFrame) bool { return of.Frame.Type == "left" && of.Frame.From == "user-1" }) &&
					hasFrame(&obsMu, &obs, func(of observedFrame) bool { return of.Frame.Type == "left" && of.Frame.From == "user-2" }) &&
					hasFrame(&obsMu, &obs, func(of observedFrame) bool { return of.Frame.Type == "left" && of.Frame.From == "user-3" })
			}); err != nil {
				return testv1.StepRunResult{}, fmt.Errorf("left events missing: %w\n%s", err, snapshotFrames(&obsMu, &obs, 120))
			}
			stage.Store("done")

			return testv1.StepRunResult{
				Report: "multiplayer REPL verified over NATS with 3 users, room switch command, and leader subtone execution",
			}, nil
		},
	})
}

func publish(nc *nats.Conn, subject string, frame repl.BusFrame) error {
	if nc == nil {
		return fmt.Errorf("nil nats connection")
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := nc.Publish(subject, data); err != nil {
		return err
	}
	return nc.FlushTimeout(2 * time.Second)
}

func hasFrame(mu *sync.Mutex, frames *[]observedFrame, predicate func(observedFrame) bool) bool {
	mu.Lock()
	defer mu.Unlock()
	for _, f := range *frames {
		if predicate(f) {
			return true
		}
	}
	return false
}

func countFrames(mu *sync.Mutex, frames *[]observedFrame, predicate func(observedFrame) bool) int {
	mu.Lock()
	defer mu.Unlock()
	n := 0
	for _, f := range *frames {
		if predicate(f) {
			n++
		}
	}
	return n
}

func waitFor(timeout time.Duration, condition func() bool) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("condition not satisfied within %s", timeout)
}

func snapshotFrames(mu *sync.Mutex, frames *[]observedFrame, max int) string {
	mu.Lock()
	defer mu.Unlock()
	if max <= 0 {
		max = 40
	}
	total := len(*frames)
	start := total - max
	if start < 0 {
		start = 0
	}
	var b strings.Builder
	for i := start; i < total; i++ {
		f := (*frames)[i]
		b.WriteString(fmt.Sprintf("%d: %s type=%s from=%s room=%s cmd=%s msg=%q\n", i, f.Subject, f.Frame.Type, f.Frame.From, f.Frame.Room, f.Frame.Command, f.Frame.Message))
	}
	return b.String()
}
