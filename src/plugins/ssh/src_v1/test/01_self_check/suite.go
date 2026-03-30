package selfcheck

import (
	"fmt"
	"strings"
	"time"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
	testv1 "dialtone/dev/plugins/test/src_v1/go"
)

type Config struct {
	Host string
}

func Register(r *testv1.Registry, cfg Config) {
	cfg = normalizeConfig(cfg)

	r.Add(testv1.Step{
		Name:    "mesh-nodes-known",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			nodes := sshv1.ListMeshNodes()
			if len(nodes) < 5 {
				return testv1.StepRunResult{}, fmt.Errorf("expected at least 5 mesh nodes, got %d", len(nodes))
			}
			if err := sc.WaitForStepMessageAfterAction("mesh-nodes-known-ok", 2*time.Second, func() error {
				sc.Infof("mesh-nodes-known-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: "mesh node list populated"}, nil
		},
	})

	r.Add(testv1.Step{
		Name:    "resolve-node-aliases",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			node, err := sshv1.ResolveMeshNode(cfg.Host)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if strings.TrimSpace(node.Name) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping: missing canonical name", cfg.Host)
			}
			if strings.TrimSpace(node.User) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping: missing user", cfg.Host)
			}
			if strings.TrimSpace(node.Port) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping: missing port", cfg.Host)
			}
			if strings.TrimSpace(node.Host) == "" {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping: empty host", cfg.Host)
			}
			if len(node.HostCandidates) == 0 {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping: no host candidates", cfg.Host)
			}
			if !looksLikeMeshHost(node.Host) {
				return testv1.StepRunResult{}, fmt.Errorf("unexpected %s mapping host=%s", cfg.Host, node.Host)
			}
			for _, candidate := range node.HostCandidates {
				if looksLikeMeshHost(candidate) {
					return testv1.StepRunResult{Report: fmt.Sprintf("mesh alias resolution verified for %s", node.Name)}, nil
				}
			}
			return testv1.StepRunResult{}, fmt.Errorf("unexpected %s host candidates=%v", cfg.Host, node.HostCandidates)
		},
	})

	r.Add(testv1.Step{
		Name:    "transport-resolution",
		Timeout: 5 * time.Second,
		RunWithContext: func(sc *testv1.StepContext) (testv1.StepRunResult, error) {
			t, err := sshv1.ResolveCommandTransport(cfg.Host)
			if err != nil {
				return testv1.StepRunResult{}, err
			}
			if t != "ssh" {
				return testv1.StepRunResult{}, fmt.Errorf("expected ssh transport for %s, got %s", cfg.Host, t)
			}
			if err := sc.WaitForStepMessageAfterAction("transport-resolution-ok", 2*time.Second, func() error {
				sc.Infof("transport-resolution-ok")
				return nil
			}); err != nil {
				return testv1.StepRunResult{}, err
			}
			return testv1.StepRunResult{Report: fmt.Sprintf("default transport resolution verified for %s", cfg.Host)}, nil
		},
	})
}

func normalizeConfig(cfg Config) Config {
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		cfg.Host = "grey"
	}
	return cfg
}

func looksLikeMeshHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	return strings.HasSuffix(host, ".shad-artichoke.ts.net") ||
		strings.HasSuffix(host, ".ts.net") ||
		strings.HasPrefix(host, "169.254.") ||
		strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "100.")
}
