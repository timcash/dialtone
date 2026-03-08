package src_v3

import (
	"flag"
	"fmt"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func handleDoctor(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 doctor", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("doctor requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	return runRemoteDoctor(node)
}

func handleLogs(args []string) error {
	fs := flag.NewFlagSet("chrome src_v3 logs", flag.ExitOnError)
	host := fs.String("host", "", "Mesh host")
	lines := fs.Int("lines", 80, "Lines to tail")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("logs requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	stdout, stderr, err := readRemoteLogs(node, *lines)
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
	host := fs.String("host", "", "Mesh host")
	_ = fs.Parse(args)
	if strings.TrimSpace(*host) == "" {
		return fmt.Errorf("reset requires --host")
	}
	node, err := sshv1.ResolveMeshNode(strings.TrimSpace(*host))
	if err != nil {
		return err
	}
	return resetRemoteHost(node)
}
