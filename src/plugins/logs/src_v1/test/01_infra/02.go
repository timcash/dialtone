package main

import (
	"fmt"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run02ErrorTopicFiltering(ctx *testCtx) (string, error) {
	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}
	nc := ctx.broker.Conn()

	sub, _ := nc.SubscribeSync("logs.error.topic")
	defer sub.Unsubscribe()

	errorTopic, err := logs.NewNATSLogger(nc, "logs.error.topic")
	if err != nil {
		return "", err
	}

	if err := errorTopic.Errorf("filtered error captured"); err != nil {
		return "", err
	}

	msg, err := sub.NextMsg(2 * time.Second)
	if err != nil || !strings.Contains(string(msg.Data), "filtered error captured") {
		return "", fmt.Errorf("filtered error verification failed: %v", err)
	}

	return "Verified error-topic filtering via NATS: logs.error.topic received the message.", nil
}
