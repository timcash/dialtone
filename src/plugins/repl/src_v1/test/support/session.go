package support

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	repl "dialtone/dev/plugins/repl/src_v1/go/repl"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

func RunSessionWithInput(ctx *testv1.StepContext, input string) (string, []string, error) {
	origStdin := os.Stdin
	origStdout := os.Stdout
	defer func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
	}()

	inR, inW, err := os.Pipe()
	if err != nil {
		return "", nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return "", nil, err
	}

	os.Stdin = inR
	os.Stdout = outW

	var outBuf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&outBuf, outR)
		done <- copyErr
	}()

	if _, err := inW.Write([]byte(input)); err != nil {
		_ = inW.Close()
		_ = inR.Close()
		_ = outW.Close()
		_ = outR.Close()
		return "", nil, err
	}
	_ = inW.Close()

	relayed := make([]string, 0, 16)
	err = repl.Start(func(category, msg string) {
		relayed = append(relayed, fmt.Sprintf("%s: %s", category, msg))
		ctx.Infof("[REPLLOG][%s] %s", category, msg)
	})

	_ = inR.Close()
	_ = outW.Close()
	copyErr := <-done
	_ = outR.Close()
	if copyErr != nil {
		return "", relayed, copyErr
	}
	if err != nil {
		return outBuf.String(), relayed, err
	}
	return outBuf.String(), relayed, nil
}

func ContainsAny(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

func RequireContainsAll(haystack string, parts []string) error {
	for _, part := range parts {
		if !strings.Contains(haystack, part) {
			return fmt.Errorf("missing expected output: %q", part)
		}
	}
	return nil
}
