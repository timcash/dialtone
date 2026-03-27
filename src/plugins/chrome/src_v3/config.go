package src_v3

import (
	"math"
	"strconv"
	"strings"
	"time"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
)

func chromeHeadlessEnabled() bool {
	return envBoolDefault("DIALTONE_CHROME_SRC_V3_HEADLESS", true)
}

func chromeCommandStepDelay() time.Duration {
	ms := envIntDefault("DIALTONE_CHROME_SRC_V3_STEP_DELAY_MS", 0)
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func durationForActionsPerSecond(actionsPerSecond float64) time.Duration {
	if actionsPerSecond <= 0 {
		return 0
	}
	nanos := float64(time.Second) / actionsPerSecond
	if nanos < 1 {
		nanos = 1
	}
	return time.Duration(math.Round(nanos))
}

func shouldDelayChromeCommand(command string) bool {
	switch strings.TrimSpace(command) {
	case "open", "goto", "tab-open", "click-aria", "type-aria", "press-enter-aria", "set-html", "reset":
		return true
	default:
		return false
	}
}

func envBoolDefault(name string, fallback bool) bool {
	raw := strings.TrimSpace(configv1.LookupEnvString(name))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envIntDefault(name string, fallback int) int {
	raw := strings.TrimSpace(configv1.LookupEnvString(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloatDefault(name string, fallback float64) float64 {
	raw := strings.TrimSpace(configv1.LookupEnvString(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}
