package main

import (
	"fmt"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

func Run02ErrorTopicFiltering(ctx *testCtx) (string, error) {
	if err := ctx.ensureBroker(); err != nil {
		return "", err
	}

	stop, err := logs.ListenToFile(ctx.broker.Conn(), "logs.error.topic", ctx.errorLog)
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

	if err := infoTopic.Infof("this should not hit error.log"); err != nil {
		return "", err
	}
	if err := errorTopic.Errorf("filtered error captured"); err != nil {
		return "", err
	}

	if err := waitForContains(ctx.errorLog, "subject=logs.error.topic", 4*time.Second); err != nil {
		return "", err
	}
	if fileContains(ctx.errorLog, "logs.info.topic") {
		return "", fmt.Errorf("error listener received non-error topic message")
	}
	return "Verified error-topic listener filtering: error.log only contains logs.error.topic records.", nil
}
