package main

import (
	"flag"
	"fmt"
	"os"

	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func main() {
	natsURL := flag.String("nats-url", "nats://127.0.0.1:4222", "NATS URL")
	subject := flag.String("subject", "logs.test.template-example", "NATS subject prefix")
	flag.Parse()

	steps := []testv1.Step{
		{
			Name: "template-step",
			RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
				sc.Logf("template plugin info")
				sc.Errorf("template plugin error")
				return testv1.StepRunResult{Report: "template step ran"}, nil
			},
		},
	}

	err := testv1.RunSuite(testv1.SuiteOptions{
		Version:     "template-plugin-example",
		NATSURL:     *natsURL,
		NATSSubject: *subject,
	}, steps)
	if err != nil {
		fmt.Printf("TEMPLATE_PLUGIN_FAIL: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("TEMPLATE_PLUGIN_PASS subject=%s\n", *subject)
}
