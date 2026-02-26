package main

import (
	"flag"
	"os"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
	"github.com/nats-io/nats.go"
)

func main() {
	logs.SetOutput(os.Stdout)

	natsURL := flag.String("nats-url", "nats://127.0.0.1:4222", "NATS URL")
	topic := flag.String("topic", "logs.example.plugin", "publish topic")
	count := flag.Int("count", 3, "message count")
	outPath := flag.String("out", "", "listener output file path")
	flag.Parse()

	if *outPath == "" {
		logs.Error("--out is required")
		return
	}

	startedEmbedded := false
	var broker *logs.EmbeddedNATS
	nc, err := nats.Connect(*natsURL, nats.Timeout(800*time.Millisecond))
	if err != nil {
		broker, err = logs.StartEmbeddedNATSOnURL(*natsURL)
		if err != nil {
			logs.Error("connect/start failed: %v", err)
			return
		}
		startedEmbedded = true
		nc = broker.Conn()
	}
	defer nc.Close()
	if broker != nil {
		defer broker.Close()
	}

	stop, err := logs.ListenToFile(nc, *topic, *outPath)
	if err != nil {
		logs.Error("listener failed: %v", err)
		return
	}
	defer func() { _ = stop() }()

	logger, err := logs.NewNATSLogger(nc, *topic)
	if err != nil {
		logs.Error("logger init failed: %v", err)
		return
	}

	for i := 1; i <= *count; i++ {
		if err := logger.Infof("example plugin message %d", i); err != nil {
			logs.Error("publish failed at %d: %v", i, err)
			return
		}
		time.Sleep(80 * time.Millisecond)
	}
	_ = nc.Flush()
	time.Sleep(200 * time.Millisecond)

	logs.Info("EXAMPLE_PLUGIN PASS started_embedded=%v nats_url=%s topic=%s count=%d out=%s", startedEmbedded, *natsURL, *topic, *count, *outPath)
}
