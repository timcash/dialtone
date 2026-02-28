package chrome

import (
	"io"
	"strconv"
	"strings"
)

func extractChromeSessionJSON(output string) string {
	const marker = "DIALTONE_CHROME_SESSION_JSON="
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, marker) {
			return strings.TrimSpace(strings.TrimPrefix(line, marker))
		}
	}
	return ""
}

func remoteShellQuote(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `'\''`)
	return "'" + v + "'"
}

func psLiteral(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `''`)
	return "'" + v + "'"
}

func psIntArray(vals []int) string {
	if len(vals) == 0 {
		return "@()"
	}
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		if v > 0 {
			parts = append(parts, strconv.Itoa(v))
		}
	}
	if len(parts) == 0 {
		return "@()"
	}
	return "@(" + strings.Join(parts, ",") + ")"
}

func normalizeRemoteDebugPorts(primary int, extras []int) []int {
	out := make([]int, 0, 1+len(extras))
	seen := map[int]struct{}{}
	add := func(v int) {
		if v <= 0 {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	add(primary)
	for _, v := range extras {
		add(v)
	}
	return out
}

func closeRemoteClosers(closers []io.Closer) {
	for _, c := range closers {
		if c != nil {
			_ = c.Close()
		}
	}
}

func outputTrim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
}
