package main

import (
	"fmt"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run01EmbeddedNATSAndPublish(ctx *testCtx) (string, error) {
	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}
	if ctx.broker.URL() == "" {
		return "", fmt.Errorf("embedded nats URL is empty")
	}

	stop, err := logs.ListenToFile(ctx.broker.Conn(), "logs.>", ctx.testLogPath)
	if err != nil {
		return "", err
	}
	ctx.addListener(stop)

	infoTopic, err := logs.NewNATSLogger(ctx.broker.Conn(), "logs.info.topic")
	if err != nil {
		return "", err
	}
	errorTopic, err := logs.NewNATSLogger(ctx.broker.Conn(), "logs.error.topic")
	if err != nil {
		return "", err
	}

	if err := infoTopic.Infof("startup ok"); err != nil {
		return "", err
	}
	if err := errorTopic.Errorf("boom happened"); err != nil {
		return "", err
	}
	if err := waitForContains(ctx.testLogPath, "subject=logs.info.topic", 4*time.Second); err != nil {
		return "", err
	}
	if err := waitForContains(ctx.testLogPath, "subject=logs.error.topic", 4*time.Second); err != nil {
		return "", err
	}

	return fmt.Sprintf("Embedded NATS started at %s and wildcard listener captured logs.info.topic + logs.error.topic.", ctx.broker.URL()), nil
}
