package cli

import (
	"fmt"
	"sort"
	"strings"

	sshv1 "dialtone/dev/plugins/ssh/src_v1/go"
)

func resolveChromeHosts(target string) ([]sshv1.MeshNode, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("host is required")
	}
	if strings.EqualFold(target, "all") {
		out := make([]sshv1.MeshNode, 0)
		for _, n := range sshv1.ListMeshNodes() {
			// GUI Chrome hosts only.
			if strings.EqualFold(strings.TrimSpace(n.OS), "windows") || strings.EqualFold(strings.TrimSpace(n.OS), "macos") {
				out = append(out, n)
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		return out, nil
	}
	parts := strings.Split(target, ",")
	out := make([]sshv1.MeshNode, 0, len(parts))
	for _, p := range parts {
		node, err := sshv1.ResolveMeshNode(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, node)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
