package logs

import (
	"fmt"
	"os"
)

func Info(format string, args ...any) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func Debug(format string, args ...any) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func Fatal(format string, args ...any) {
	fmt.Printf("[FATAL] "+format+"\n", args...)
	os.Exit(1)
}
