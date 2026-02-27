package ssh

import (
	"fmt"
	"strings"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type BootstrapOptions struct {
	Node        string
	Source      string
	Dest        string
	Delete      bool
	NoSync      bool
	InstallCmds []string
	VerifyCmd   string
}

func Bootstrap(opts BootstrapOptions) error {
	targets, err := resolveBootstrapTargets(opts.Node)
	if err != nil {
		return err
	}
	failed := 0
	for _, node := range targets {
		logs.Raw("== %s ==", node.Name)
		dest := strings.TrimSpace(opts.Dest)
		if dest == "" {
			dest = defaultSyncDestForNode(node)
		}
		if !opts.NoSync {
			if err := SyncCode(SyncCodeOptions{
				Node:   node.Name,
				Source: opts.Source,
				Dest:   dest,
				Delete: opts.Delete,
			}); err != nil {
				failed++
				logs.Raw("ERROR: sync failed on %s: %v", node.Name, err)
				continue
			}
		}

		cmd := buildBootstrapCommand(dest, opts.InstallCmds, opts.VerifyCmd)
		out, runErr := RunNodeCommand(node.Name, cmd, CommandOptions{})
		if strings.TrimSpace(out) != "" {
			logs.Raw("%s", strings.TrimSpace(out))
		}
		if runErr != nil {
			failed++
			logs.Raw("ERROR: bootstrap command failed on %s: %v", node.Name, runErr)
			continue
		}
		logs.Raw("bootstrap completed on %s (%s)", node.Name, dest)
	}
	if failed > 0 {
		return fmt.Errorf("bootstrap finished with %d failures", failed)
	}
	return nil
}

func resolveBootstrapTargets(node string) ([]MeshNode, error) {
	name := strings.TrimSpace(node)
	if name == "" {
		return nil, fmt.Errorf("node is required")
	}
	if name == "all" {
		return ListMeshNodes(), nil
	}
	n, err := ResolveMeshNode(name)
	if err != nil {
		return nil, err
	}
	return []MeshNode{n}, nil
}

func buildBootstrapCommand(dest string, installCmds []string, verifyCmd string) string {
	lines := []string{
		"set -euo pipefail",
		"repo=" + shellQuoteBootstrap(dest),
		"mkdir -p \"$repo\"",
		"cd \"$repo\"",
		"if [ ! -x \"./dialtone.sh\" ]; then echo \"dialtone.sh not found in $repo\"; exit 1; fi",
		"if [ -z \"${DIALTONE_ENV:-}\" ]; then export DIALTONE_ENV=\"$HOME/.dialtone_env\"; fi",
	}
	for _, cmd := range installCmds {
		c := strings.TrimSpace(cmd)
		if c == "" {
			continue
		}
		lines = append(lines, c)
	}
	verify := strings.TrimSpace(verifyCmd)
	if verify != "" {
		lines = append(lines, verify)
	}
	return strings.Join(lines, "; ")
}

func shellQuoteBootstrap(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `'`, `'\''`)
	return "'" + v + "'"
}
