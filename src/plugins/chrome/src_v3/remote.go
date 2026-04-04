package src_v3

import (
	"flag"
	"fmt"
	"strings"
)

func handleDoctor(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 doctor", flag.ExitOnError)
	host := fs.String("host", "", chromeHostFlagUsage)
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	return doctorTarget(strings.TrimSpace(*host), strings.TrimSpace(*role))
}

func handleLogs(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 logs", flag.ExitOnError)
	host := fs.String("host", "", chromeHostFlagUsage)
	role := fs.String("role", defaultRole, "Chrome role")
	lines := fs.Int("lines", 80, "Lines to tail")
	_ = fs.Parse(args)
	stdout, stderr, err := readTargetLogs(strings.TrimSpace(*host), strings.TrimSpace(*role), *lines)
	if err != nil {
		return err
	}
	if strings.TrimSpace(stdout) != "" {
		fmt.Println("STDOUT LOG")
		fmt.Println(stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		fmt.Println("STDERR LOG")
		fmt.Println(stderr)
	}
	return nil
}

func handleReset(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 reset", flag.ExitOnError)
	host := fs.String("host", "", chromeHostFlagUsage)
	role := fs.String("role", defaultRole, "Chrome role")
	_ = fs.Parse(args)
	return resetTarget(strings.TrimSpace(*host), strings.TrimSpace(*role))
}
