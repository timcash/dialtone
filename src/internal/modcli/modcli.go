package modcli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const DefaultShell = "default"

func FindRepoRoot() (string, error) {
	if envRoot := strings.TrimSpace(os.Getenv("DIALTONE_REPO_ROOT")); envRoot != "" {
		candidate := filepath.Clean(envRoot)
		if IsRepoRoot(candidate) {
			return candidate, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd = filepath.Clean(cwd)
	for {
		if IsRepoRoot(cwd) {
			return cwd, nil
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
	return "", fmt.Errorf("unable to locate repo root from %s", cwd)
}

func IsRepoRoot(candidate string) bool {
	if _, err := os.Stat(filepath.Join(candidate, "dialtone_mod")); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(candidate, "src", "go.mod"))
	return err == nil
}

func SrcRoot(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "src")
}

func ModDir(repoRoot, modName, version string) string {
	return filepath.Join(SrcRoot(repoRoot), "mods", strings.TrimSpace(modName), strings.TrimSpace(version))
}

func CLIDir(repoRoot, modName, version string) string {
	return filepath.Join(ModDir(repoRoot, modName, version), "cli")
}

func BinDir(repoRoot, modName, version string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "bin", "mods", strings.TrimSpace(modName), strings.TrimSpace(version))
}

func EnsureBinDir(repoRoot, modName, version string) (string, error) {
	dir := BinDir(repoRoot, modName, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create bin dir: %w", err)
	}
	return dir, nil
}

func BuildOutputPath(repoRoot, modName, version, artifact string) (string, error) {
	dir, err := EnsureBinDir(repoRoot, modName, version)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(artifact)
	if name == "" {
		name = strings.TrimSpace(modName)
	}
	return filepath.Join(dir, name), nil
}

func GoBuildCommand(repoRoot, shellName, outputPath, packagePath string) *exec.Cmd {
	cmd := NixDevelopCommand(repoRoot, shellName, "go", "build", "-o", outputPath, packagePath)
	cmd.Dir = SrcRoot(repoRoot)
	return cmd
}

func GoTestCommand(repoRoot, shellName string, packagePaths ...string) *exec.Cmd {
	args := []string{"go", "test"}
	args = append(args, packagePaths...)
	cmd := NixDevelopCommand(repoRoot, shellName, args...)
	cmd.Dir = SrcRoot(repoRoot)
	return cmd
}

func NixDevelopCommand(repoRoot, shellName string, command ...string) *exec.Cmd {
	if strings.TrimSpace(shellName) == "" {
		shellName = DefaultShell
	}
	if os.Getenv("DIALTONE_NIX_ACTIVE") == "1" {
		argv := append([]string(nil), command...)
		if len(argv) > 0 && argv[0] == "go" {
			if goBin := strings.TrimSpace(os.Getenv("DIALTONE_GO_BIN")); goBin != "" {
				argv[0] = goBin
			}
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = append(
			os.Environ(),
			"DIALTONE_REPO_ROOT="+repoRoot,
			"DIALTONE_NIX_SHELL="+shellName,
		)
		return cmd
	}

	args := []string{}
	if strings.TrimSpace(os.Getenv("DIALTONE_NIX_OFFLINE")) == "1" {
		args = append(args, "--offline")
	}
	args = append(args,
		"--extra-experimental-features",
		"nix-command flakes",
		"develop",
		"path:"+repoRoot+"#"+shellName,
		"--command",
	)
	args = append(args, command...)
	cmd := exec.Command("nix", args...)
	cmd.Dir = repoRoot
	cmd.Env = append(
		os.Environ(),
		"DIALTONE_NIX_ACTIVE=1",
		"DIALTONE_NIX_OFFLINE="+strings.TrimSpace(os.Getenv("DIALTONE_NIX_OFFLINE")),
		"DIALTONE_NIX_SHELL="+shellName,
		"DIALTONE_REPO_ROOT="+repoRoot,
	)
	return cmd
}

func CurrentTmuxTarget(explicitTarget, paneID string, lookup func(string) (string, error)) string {
	if target := strings.TrimSpace(explicitTarget); target != "" {
		return target
	}
	paneID = strings.TrimSpace(paneID)
	if paneID == "" || lookup == nil {
		return ""
	}
	target, err := lookup(paneID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(target)
}

func NormalizeOptionalPathArg(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func CollectGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "target", ".zig-cache", "zig-out":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
