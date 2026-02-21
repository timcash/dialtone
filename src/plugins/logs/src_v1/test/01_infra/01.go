package main

import (
	"fmt"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run01EmbeddedNATSAndPublish(ctx *testCtx) (string, error) {
	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}
	nc := ctx.broker.Conn()

	// Subscriptions MUST happen before publish
	subInfo, _ := nc.SubscribeSync("logs.info.topic")
	defer subInfo.Unsubscribe()
	subError, _ := nc.SubscribeSync("logs.error.topic")
	defer subError.Unsubscribe()

	infoTopic, err := logs.NewNATSLogger(nc, "logs.info.topic")
	if err != nil {
		return "", err
	}
	errorTopic, err := logs.NewNATSLogger(nc, "logs.error.topic")
	if err != nil {
		return "", err
	}

	if err := infoTopic.Infof("startup ok"); err != nil {
		return "", err
	}
	if err := errorTopic.Errorf("boom happened"); err != nil {
		return "", err
	}

	// Verify
	msg, err := subInfo.NextMsg(2 * time.Second)
	if err != nil || !strings.Contains(string(msg.Data), "startup ok") {
		return "", fmt.Errorf("info message verification failed: %v", err)
	}
	msg, err = subError.NextMsg(2 * time.Second)
	if err != nil || !strings.Contains(string(msg.Data), "boom happened") {
		return "", fmt.Errorf("error message verification failed: %v", err)
	}

	return fmt.Sprintf("Embedded NATS started at %s and NATS messages verified.", ctx.broker.URL()), nil
}
