package logs

import (
	"fmt"
	"io"
	"os"
)

var logOutput io.Writer = io.Discard

func SetOutput(w io.Writer) {
	logOutput = w
}

func Info(format string, args ...any) {
	fmt.Fprintf(logOutput, "[INFO] "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(logOutput, "[ERROR] "+format+"\n", args...)
}

func Errorf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	Error("%s", msg)
	return fmt.Errorf("%s", msg)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(logOutput, "[WARN] "+format+"\n", args...)
}

func Debug(format string, args ...any) {
	fmt.Fprintf(logOutput, "[DEBUG] "+format+"\n", args...)
}

func Fatal(format string, args ...any) {
	fmt.Fprintf(logOutput, "[FATAL] "+format+"\n", args...)
	os.Exit(1)
}
