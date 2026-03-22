package nixplan

import (
	"bufio"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dialtone/dev/internal/modstate"
)

type Plan struct {
	FlakeShell string
	Packages   []string
}

func BuildPlan(db *sql.DB, repoRoot, modName, version, goos string) (Plan, error) {
	packages := []string{"nixpkgs#bashInteractive", "nixpkgs#git", "nixpkgs#go_1_25"}
	add := func(pkg string) {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" {
			return
		}
		for _, existing := range packages {
			if existing == pkg {
				return
			}
		}
		packages = append(packages, pkg)
	}

	flakeShell := fallbackFlakeShell(modName, version)
	if db != nil {
		if err := modstate.EnsureSchema(db); err != nil {
			return Plan{}, err
		}
		if launchConfig, err := modstate.LoadLaunchConfig(db, modName, version); err == nil && strings.TrimSpace(launchConfig.FlakeShell) != "" {
			flakeShell = strings.TrimSpace(launchConfig.FlakeShell)
		}
		if records, err := modstate.LoadNixPackages(db, modName, version); err == nil && len(records) > 0 {
			for _, record := range records {
				if selectorMatches(record.Selector, goos) {
					add(record.PackageRef)
				}
			}
			sort.Strings(packages)
			return Plan{FlakeShell: flakeShell, Packages: packages}, nil
		}
	}

	manifestPath := filepath.Join(repoRoot, "src", "mods", modName, version, "nix.packages")
	file, err := os.Open(manifestPath)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			raw := strings.TrimSpace(scanner.Text())
			if raw == "" || strings.HasPrefix(raw, "#") {
				continue
			}
			selector := ""
			pkg := raw
			if before, after, ok := strings.Cut(raw, ":"); ok {
				selector = strings.TrimSpace(before)
				pkg = strings.TrimSpace(after)
			}
			if selectorMatches(selector, goos) {
				add(pkg)
			}
		}
		if err := scanner.Err(); err != nil {
			return Plan{}, err
		}
	}

	sort.Strings(packages)
	return Plan{FlakeShell: flakeShell, Packages: packages}, nil
}

func fallbackFlakeShell(modName, version string) string {
	if strings.TrimSpace(modName) == "" || strings.TrimSpace(version) == "" {
		return "default"
	}
	return "default"
}

func selectorMatches(selector, goos string) bool {
	switch strings.TrimSpace(selector) {
	case "", "all":
		return true
	case "darwin":
		return strings.EqualFold(strings.TrimSpace(goos), "darwin")
	case "linux":
		return strings.EqualFold(strings.TrimSpace(goos), "linux")
	default:
		return false
	}
}
