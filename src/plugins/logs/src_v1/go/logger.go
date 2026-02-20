package logs

import (
	"fmt"
	"io"
	"os"
)

var logOutput io.Writer = os.Stdout

func SetOutput(w io.Writer) {
	logOutput = w
}

func Info(format string, args ...any) {
	fmt.Fprintf(logOutput, "[INFO] "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(logOutput, "[ERROR] "+format+"\n", args...)
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
