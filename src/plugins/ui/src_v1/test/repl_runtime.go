package test

import (
	configv1 "dialtone/dev/plugins/config/src_v1/go"
	"dialtone/dev/plugins/logs/src_v1/go"
)

func ResolveSuiteNATSURL() string {
	return configv1.ResolveREPLNATSURL()
}

func ReplIndexInfof(format string, args ...any) {
	logs.Info("DIALTONE_INDEX: "+format, args...)
}
