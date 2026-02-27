package test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type TemplateReportOptions struct {
	Title   string
	Version string
	Runner  string
}

type parsedTemplateStep struct {
	Name        string
	Passed      bool
	Duration    string
	Report      string
	Error       string
	Logs        []string
	Errors      []string
	BrowserLogs []string
	Screenshots []string
}

func RenderTemplateReport(rawPath, outPath string, opts TemplateReportOptions) error {
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		return err
	}
	totalDuration, status, steps := parseRawReportMarkdown(string(raw))
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Plugin Test Report"
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "unknown"
	}
	runner := strings.TrimSpace(opts.Runner)
	if runner == "" {
		runner = "test/src_v1"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", title))
	sb.WriteString(fmt.Sprintf("**Generated at:** %s\n", time.Now().Format(time.RFC1123Z)))
	sb.WriteString(fmt.Sprintf("**Version:** `%s`\n", version))
	sb.WriteString(fmt.Sprintf("**Runner:** `%s`\n", runner))
	if status == "PASSED" {
		sb.WriteString("**Status:** ✅ PASS\n")
	} else {
		sb.WriteString("**Status:** ❌ FAIL\n")
	}
	sb.WriteString(fmt.Sprintf("**Total Time:** `%s`\n\n", totalDuration))

	sb.WriteString("## Test Steps\n\n")
	sb.WriteString("| Step | Result | Duration |\n")
	sb.WriteString("|---|---|---|\n")
	for _, st := range steps {
		result := "✅ PASS"
		if !st.Passed {
			result = "❌ FAIL"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | `%s` |\n", st.Name, result, st.Duration))
	}
	sb.WriteString("\n## Step Details\n\n")
	for _, st := range steps {
		sb.WriteString(fmt.Sprintf("## %s\n\n", st.Name))
		sb.WriteString("### Results\n\n")
		sb.WriteString("```text\n")
		if st.Passed {
			sb.WriteString("result: PASS\n")
		} else {
			sb.WriteString("result: FAIL\n")
		}
		sb.WriteString(fmt.Sprintf("duration: %s\n", st.Duration))
		if strings.TrimSpace(st.Report) != "" {
			sb.WriteString(fmt.Sprintf("report: %s\n", st.Report))
		}
		if strings.TrimSpace(st.Error) != "" {
			sb.WriteString(fmt.Sprintf("error: %s\n", st.Error))
		}
		sb.WriteString("```\n\n")
		if len(st.Logs) > 0 {
			sb.WriteString("### Logs\n\n")
			sb.WriteString("```text\n")
			sb.WriteString("logs:\n")
			for _, line := range st.Logs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		}
		if len(st.Errors) > 0 {
			sb.WriteString("### Errors\n\n")
			sb.WriteString("```text\n")
			sb.WriteString("errors:\n")
			for _, line := range st.Errors {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		}
		sb.WriteString("### Browser Logs\n\n")
		sb.WriteString("```text\n")
		sb.WriteString("browser_logs:\n")
		if len(st.BrowserLogs) == 0 {
			sb.WriteString("<empty>\n")
		} else {
			for _, line := range st.BrowserLogs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("```\n\n")
		if len(st.Screenshots) > 0 {
			sb.WriteString("### Screenshots\n\n")
		}
		links := make([]string, 0, len(st.Screenshots))
		labels := make([]string, 0, len(st.Screenshots))
		for _, shot := range st.Screenshots {
			base := filepath.Base(shot)
			link := screenshotMarkdownLink(outPath, shot)
			links = append(links, link)
			labels = append(labels, base)
		}
		if len(links) == 1 {
			sb.WriteString(fmt.Sprintf("![%s](%s)\n", labels[0], links[0]))
		} else if len(links) > 1 {
			sb.WriteString("|  |  |  |  |\n")
			sb.WriteString("|---|---|---|---|\n")
			for i := 0; i < len(links); i += 4 {
				cells := []string{"", "", "", ""}
				for j := 0; j < 4; j++ {
					idx := i + j
					if idx >= len(links) {
						break
					}
					cells[j] = fmt.Sprintf("![%s](%s)", labels[idx], links[idx])
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", cells[0], cells[1], cells[2], cells[3]))
			}
		}
		if len(st.Screenshots) > 0 {
			sb.WriteString("\n")
		}
	}

	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}

func screenshotMarkdownLink(reportPath, shot string) string {
	base := filepath.Base(strings.TrimSpace(shot))
	if base == "" {
		return "screenshots"
	}
	absShot := filepath.Join(filepath.Dir(reportPath), "screenshots", base)
	norm := filepath.ToSlash(absShot)
	if idx := strings.Index(norm, "/src/"); idx >= 0 {
		return norm[idx:]
	}
	return "screenshots/" + base
}

func parseRawReportMarkdown(raw string) (string, string, []parsedTemplateStep) {
	totalDuration := ""
	status := "FAILED"
	stepHeader := regexp.MustCompile(`^###\s+\d+\.\s+(✅|❌)\s+(.+)$`)
	durationRe := regexp.MustCompile(`^- \*\*Duration\*\*:\s+(.+)$`)
	reportRe := regexp.MustCompile(`^- \*\*Report\*\*:\s+(.+)$`)
	errorRe := regexp.MustCompile(`^- \*\*Error\*\*:\s+` + "`" + `(.+)` + "`" + `$`)
	totalRe := regexp.MustCompile(`^- \*\*Total Duration\*\*:\s+(.+)$`)
	statusRe := regexp.MustCompile(`^- \*\*Status\*\*:\s+(.+)$`)
	screenRe := regexp.MustCompile(`^!\[[^\]]*\]\(([^)]+)\)$`)

	var steps []parsedTemplateStep
	current := -1
	captureBlock := ""
	inFence := false
	s := bufio.NewScanner(strings.NewReader(raw))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "#### Logs" {
			captureBlock = "logs"
			continue
		}
		if line == "#### Errors" {
			captureBlock = "errors"
			continue
		}
		if line == "#### Browser Logs" {
			captureBlock = "browser_logs"
			continue
		}
		if line == "```text" && (captureBlock == "logs" || captureBlock == "errors" || captureBlock == "browser_logs") {
			inFence = true
			continue
		}
		if line == "```" && inFence {
			inFence = false
			captureBlock = ""
			continue
		}
		if m := totalRe.FindStringSubmatch(line); len(m) == 2 {
			totalDuration = strings.TrimSpace(m[1])
			continue
		}
		if m := statusRe.FindStringSubmatch(line); len(m) == 2 {
			status = strings.TrimSpace(m[1])
			continue
		}
		if m := stepHeader.FindStringSubmatch(line); len(m) == 3 {
			steps = append(steps, parsedTemplateStep{
				Name:   strings.TrimSpace(m[2]),
				Passed: m[1] == "✅",
			})
			current = len(steps) - 1
			continue
		}
		if current < 0 {
			continue
		}
		if inFence && captureBlock == "logs" {
			if line != "" {
				steps[current].Logs = append(steps[current].Logs, line)
			}
			continue
		}
		if inFence && captureBlock == "errors" {
			if line != "" {
				steps[current].Errors = append(steps[current].Errors, line)
			}
			continue
		}
		if inFence && captureBlock == "browser_logs" {
			if line != "" && line != "<empty>" {
				steps[current].BrowserLogs = append(steps[current].BrowserLogs, line)
			}
			continue
		}
		if m := durationRe.FindStringSubmatch(line); len(m) == 2 {
			steps[current].Duration = strings.TrimSpace(m[1])
			continue
		}
		if m := reportRe.FindStringSubmatch(line); len(m) == 2 {
			steps[current].Report = strings.TrimSpace(m[1])
			continue
		}
		if m := errorRe.FindStringSubmatch(line); len(m) == 2 {
			steps[current].Error = strings.TrimSpace(m[1])
			continue
		}
		if m := screenRe.FindStringSubmatch(line); len(m) == 2 {
			steps[current].Screenshots = append(steps[current].Screenshots, strings.TrimSpace(m[1]))
		}
	}
	if strings.TrimSpace(totalDuration) == "" {
		totalDuration = "unknown"
	}
	return totalDuration, status, steps
}
