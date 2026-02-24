package multiplayer

import (
	"fmt"
	"os"
	"strings"
)

func resolveHostSpecs() ([]hostSpec, error) {
	namesRaw := strings.TrimSpace(os.Getenv("REPL_MULTIPLAYER_HOSTS"))
	if namesRaw == "" {
		namesRaw = "chroma,darkmac,robot"
	}
	names := strings.Split(namesRaw, ",")
	out := make([]hostSpec, 0, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		switch strings.ToLower(name) {
		case "robot":
			h := hostSpec{
				Name: "robot",
				Host: strings.TrimSpace(os.Getenv("ROBOT_HOST")),
				User: strings.TrimSpace(os.Getenv("ROBOT_USER")),
				Pass: strings.TrimSpace(os.Getenv("ROBOT_PASSWORD")),
			}
			if h.Host == "" || h.User == "" || h.Pass == "" {
				return nil, fmt.Errorf("robot host credentials missing (ROBOT_HOST/ROBOT_USER/ROBOT_PASSWORD)")
			}
			out = append(out, h)
		case "chroma":
			h := hostSpec{
				Name: "chroma",
				Host: firstNonEmpty(os.Getenv("CHROMA_HOST"), "chroma"),
				User: firstNonEmpty(os.Getenv("CHROMA_USER"), "dev"),
				Pass: strings.TrimSpace(os.Getenv("CHROMA_PASSWORD")),
			}
			if h.Pass == "" {
				return nil, fmt.Errorf("chroma password missing (CHROMA_PASSWORD)")
			}
			out = append(out, h)
		case "darkmac":
			h := hostSpec{
				Name: "darkmac",
				Host: firstNonEmpty(os.Getenv("DARKMAC_HOST"), "darkmac"),
				User: firstNonEmpty(os.Getenv("DARKMAC_USER"), "tim"),
				Pass: strings.TrimSpace(os.Getenv("DARKMAC_PASSWORD")),
			}
			if h.Pass == "" {
				return nil, fmt.Errorf("darkmac password missing (DARKMAC_PASSWORD)")
			}
			out = append(out, h)
		default:
			return nil, fmt.Errorf("unsupported host name %q in REPL_MULTIPLAYER_HOSTS", name)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no hosts configured")
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}
