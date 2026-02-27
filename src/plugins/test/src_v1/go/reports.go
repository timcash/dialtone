package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type StepResult struct {
	Step        Step
	Error       error
	Result      StepRunResult
	Start       time.Time
	End         time.Time
	Logs        []string
	Errors      []string
	BrowserLogs []string
}

func generateReport(opts SuiteOptions, results []StepResult, totalDuration time.Duration) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Test Report: %s\n\n", opts.Version))
	sb.WriteString(fmt.Sprintf("- **Date**: %s\n", time.Now().Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("- **Total Duration**: %v\n\n", totalDuration))

	sb.WriteString("## Summary\n\n")
	passed := 0
	for _, r := range results {
		if r.Error == nil {
			passed++
		}
	}
	sb.WriteString(fmt.Sprintf("- **Steps**: %d / %d passed\n", passed, len(results)))
	status := "PASSED"
	if passed < len(results) {
		status = "FAILED"
	}
	sb.WriteString(fmt.Sprintf("- **Status**: %s\n\n", status))

	sb.WriteString("## Details\n\n")
	for i, r := range results {
		icon := "✅"
		if r.Error != nil {
			icon = "❌"
		}
		sb.WriteString(fmt.Sprintf("### %d. %s %s\n\n", i+1, icon, r.Step.Name))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n", r.End.Sub(r.Start)))
		if r.Error != nil {
			sb.WriteString(fmt.Sprintf("- **Error**: `%v`\n", r.Error))
		}
		if r.Result.Report != "" {
			sb.WriteString(fmt.Sprintf("- **Report**: %s\n", r.Result.Report))
		}
		if len(r.Logs) > 0 {
			sb.WriteString("\n#### Logs\n\n```text\n")
			for _, line := range r.Logs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
		if len(r.Errors) > 0 {
			sb.WriteString("\n#### Errors\n\n```text\n")
			for _, line := range r.Errors {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
		sb.WriteString("\n#### Browser Logs\n\n```text\n")
		if len(r.BrowserLogs) == 0 {
			sb.WriteString("<empty>\n")
		} else {
			for _, line := range r.BrowserLogs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("```\n")

		if len(r.Step.Screenshots) > 0 {
			sb.WriteString("\n#### Screenshots\n\n")
			for _, s := range r.Step.Screenshots {
				fname := filepath.Base(s)
				sb.WriteString(fmt.Sprintf("![%s](screenshots/%s)\n", fname, fname))
			}
		}
		sb.WriteString("\n---\n\n")
	}

	return os.WriteFile(opts.ReportPath, []byte(sb.String()), 0644)
}

func generateReports(opts SuiteOptions, results []StepResult, totalDuration time.Duration) error {
	format := strings.ToLower(strings.TrimSpace(opts.ReportFormat))
	if format != "template" {
		if err := generateReport(opts, results, totalDuration); err != nil {
			return err
		}
		return generateErrorsReport(opts, results, totalDuration)
	}
	reportPath := strings.TrimSpace(opts.ReportPath)
	if reportPath == "" {
		return nil
	}
	rawPath := strings.TrimSpace(opts.RawReportPath)
	if rawPath == "" {
		ext := filepath.Ext(reportPath)
		base := strings.TrimSuffix(reportPath, ext)
		if ext == "" {
			rawPath = reportPath + "_RAW.md"
		} else {
			rawPath = base + "_RAW" + ext
		}
	}
	rawOpts := opts
	rawOpts.ReportPath = rawPath
	rawOpts.ReportFormat = ""
	rawOpts.RawReportPath = ""
	if err := generateReport(rawOpts, results, totalDuration); err != nil {
		return err
	}
	if err := RenderTemplateReport(rawPath, reportPath, TemplateReportOptions{
		Title:   strings.TrimSpace(opts.ReportTitle),
		Version: strings.TrimSpace(opts.Version),
		Runner:  strings.TrimSpace(opts.ReportRunner),
	}); err != nil {
		return err
	}
	return generateErrorsReport(opts, results, totalDuration)
}

func generateErrorsReport(opts SuiteOptions, results []StepResult, totalDuration time.Duration) error {
	reportPath := strings.TrimSpace(opts.ReportPath)
	if reportPath == "" {
		return nil
	}
	errorsPath := filepath.Join(filepath.Dir(reportPath), "ERRORS.md")
	var sb strings.Builder
	sb.WriteString("# Error Report\n\n")
	sb.WriteString(fmt.Sprintf("- **Date**: %s\n", time.Now().Format(time.RFC1123)))
	sb.WriteString(fmt.Sprintf("- **Suite**: %s\n", strings.TrimSpace(opts.Version)))
	sb.WriteString(fmt.Sprintf("- **Total Duration**: %v\n\n", totalDuration))

	type errorStep struct {
		Index int
		Step  StepResult
	}
	steps := make([]errorStep, 0)
	for i, r := range results {
		if !stepHasErrors(r) {
			continue
		}
		steps = append(steps, errorStep{Index: i + 1, Step: r})
	}
	sb.WriteString(fmt.Sprintf("- **Error Steps**: %d / %d\n\n", len(steps), len(results)))

	if len(steps) == 0 {
		sb.WriteString("No errors captured.\n")
		return os.WriteFile(errorsPath, []byte(sb.String()), 0644)
	}

	for _, es := range steps {
		r := es.Step
		sb.WriteString(fmt.Sprintf("## %d. %s\n\n", es.Index, r.Step.Name))
		sb.WriteString(fmt.Sprintf("- **Duration**: %v\n", r.End.Sub(r.Start)))
		if r.Error != nil {
			sb.WriteString(fmt.Sprintf("- **Step Error**: `%v`\n", r.Error))
		}
		stepErrs := extractErrorLines(r.Errors)
		if len(stepErrs) > 0 {
			sb.WriteString("\n### Step Errors\n\n```text\n")
			for _, line := range stepErrs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
		browserErrs := extractBrowserErrorLines(r.BrowserLogs)
		if len(browserErrs) > 0 {
			sb.WriteString("\n### Browser Errors\n\n```text\n")
			for _, line := range browserErrs {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("```\n")
		}
		sb.WriteString("\n---\n\n")
	}
	return os.WriteFile(errorsPath, []byte(sb.String()), 0644)
}

func stepHasErrors(r StepResult) bool {
	if r.Error != nil {
		return true
	}
	if len(extractErrorLines(r.Errors)) > 0 {
		return true
	}
	if len(extractBrowserErrorLines(r.BrowserLogs)) > 0 {
		return true
	}
	return false
}

func extractErrorLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func extractBrowserErrorLines(lines []string) []string {
	out := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "exception") || strings.Contains(lower, "error") || strings.Contains(lower, "fail") {
			out = append(out, trimmed)
		}
	}
	return out
}
