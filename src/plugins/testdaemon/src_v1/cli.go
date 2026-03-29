package testdaemon

import (
	"fmt"
	"strings"
)

type ExitStatusError struct {
	Code    int
	Message string
}

func (e *ExitStatusError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return strings.TrimSpace(e.Message)
	}
	return fmt.Sprintf("exit status %d", e.Code)
}

func Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh testdaemon src_v1 <command> [args]")
	}

	switch strings.TrimSpace(args[0]) {
	case "format", "fmt":
		return RunFormat(args[1:])
	case "build":
		return RunBuild(args[1:])
	case "test":
		return RunTest(args[1:])
	case "run":
		return RunRun(args[1:])
	case "service":
		return RunService(args[1:])
	case "emit-progress":
		return RunEmitProgress(args[1:])
	case "sleep":
		return RunSleep(args[1:])
	case "exit-code":
		return RunExitCode(args[1:])
	case "panic":
		return RunPanic(args[1:])
	case "crash":
		return RunCrash(args[1:])
	case "hang":
		return RunHang(args[1:])
	case "heartbeat":
		return RunHeartbeat(args[1:])
	case "shutdown":
		return RunShutdown(args[1:])
	case "daemon":
		return RunDaemon(args[1:])
	case "help", "-h", "--help":
		return nil
	default:
		return fmt.Errorf("unsupported testdaemon src_v1 command %q", strings.TrimSpace(args[0]))
	}
}
