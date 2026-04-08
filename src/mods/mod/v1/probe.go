package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func runProbe(args []string) error {
	return runProbeWithIO(args, os.Stdout, os.Stderr)
}

func runProbeWithIO(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("mods probe", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	mode := fs.String("mode", "success", "Probe mode: success|sleep|fail|background")
	sleepMS := fs.Int("sleep-ms", 0, "Milliseconds to sleep before finishing")
	label := fs.String("label", "dialtone-probe", "Human-readable label written into probe output")
	backgroundFile := fs.String("background-file", "", "Marker file that background mode writes when the detached task finishes")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("probe does not accept positional arguments")
	}
	if *sleepMS < 0 {
		return errors.New("--sleep-ms must be non-negative")
	}

	config := probeConfig{
		mode:           strings.TrimSpace(*mode),
		sleepMS:        *sleepMS,
		label:          strings.TrimSpace(*label),
		backgroundFile: strings.TrimSpace(*backgroundFile),
	}
	if config.label == "" {
		config.label = "dialtone-probe"
	}
	return executeProbe(config, stdout, stderr)
}

type probeConfig struct {
	mode           string
	sleepMS        int
	label          string
	backgroundFile string
}

func executeProbe(config probeConfig, stdout, stderr io.Writer) error {
	mode := strings.ToLower(strings.TrimSpace(config.mode))
	switch mode {
	case "success", "sleep", "fail", "background":
	default:
		return fmt.Errorf("unsupported --mode %q", config.mode)
	}
	sleep := time.Duration(config.sleepMS) * time.Millisecond
	startedAt := time.Now().UTC().Format(time.RFC3339)
	pid := os.Getpid()

	writeProbeLine(stdout, "probe_mode", mode)
	writeProbeLine(stdout, "probe_label", config.label)
	writeProbeLine(stdout, "probe_pid", strconv.Itoa(pid))
	writeProbeLine(stdout, "probe_started_at", startedAt)
	writeProbeLine(stdout, "probe_sleep_ms", strconv.Itoa(config.sleepMS))

	switch mode {
	case "success":
		writeProbeLine(stdout, "probe_result", "success")
		return nil
	case "sleep":
		time.Sleep(sleep)
		writeProbeLine(stdout, "probe_finished_at", time.Now().UTC().Format(time.RFC3339))
		writeProbeLine(stdout, "probe_result", "success")
		return nil
	case "fail":
		time.Sleep(sleep)
		writeProbeLine(stdout, "probe_finished_at", time.Now().UTC().Format(time.RFC3339))
		writeProbeLine(stdout, "probe_result", "failure")
		writeProbeLine(stdout, "probe_error", "requested failure")
		fmt.Fprintf(stderr, "probe %s requested failure\n", config.label)
		return fmt.Errorf("probe %s requested failure", config.label)
	case "background":
		backgroundFile := config.backgroundFile
		if backgroundFile == "" {
			backgroundFile = filepath.Join(os.TempDir(), fmt.Sprintf("dialtone-probe-%d.txt", time.Now().UnixNano()))
		}
		backgroundPID, err := startProbeBackgroundWriter(backgroundFile, config.label, sleep)
		if err != nil {
			return err
		}
		writeProbeLine(stdout, "probe_background_file", backgroundFile)
		writeProbeLine(stdout, "probe_background_pid", strconv.Itoa(backgroundPID))
		writeProbeLine(stdout, "probe_finished_at", time.Now().UTC().Format(time.RFC3339))
		writeProbeLine(stdout, "probe_result", "background-started")
		return nil
	default:
		return fmt.Errorf("unsupported --mode %q", config.mode)
	}
}

func startProbeBackgroundWriter(path, label string, sleep time.Duration) (int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		script := fmt.Sprintf(
			"$path=%s; Start-Sleep -Milliseconds %d; Set-Content -LiteralPath $path -Value @(%s,%s)",
			probePowerShellQuote(path),
			sleep.Milliseconds(),
			probePowerShellQuote("probe_background_done\t"+label),
			probePowerShellQuote("probe_label\t"+label),
		)
		cmd = exec.Command("powershell.exe", "-NoProfile", "-Command", script)
	} else {
		script := fmt.Sprintf(
			"sleep %s; printf 'probe_background_done\\t%s\\n' > %s; printf 'probe_label\\t%s\\n' >> %s",
			probeSleepLiteral(sleep),
			probeShellEscapeLiteral(label),
			probeShellQuote(path),
			probeShellEscapeLiteral(label),
			probeShellQuote(path),
		)
		cmd = exec.Command("sh", "-lc", script)
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = nil
	setDetachedProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	if cmd.Process == nil {
		return 0, errors.New("background probe process did not start")
	}
	return cmd.Process.Pid, nil
}

func writeProbeLine(w io.Writer, key, value string) {
	fmt.Fprintf(w, "%s\t%s\n", strings.TrimSpace(key), strings.TrimSpace(value))
}

func probeSleepLiteral(delay time.Duration) string {
	if delay <= 0 {
		return "0"
	}
	return fmt.Sprintf("%.3f", delay.Seconds())
}

func probeShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func probeShellEscapeLiteral(value string) string {
	return strings.ReplaceAll(value, "'", `'"'"'`)
}

func probePowerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
