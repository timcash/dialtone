package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type Issue struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
	Author    GHAuthor  `json:"author"`
	Labels    []GHLabel `json:"labels"`
}

type GHAuthor struct {
	Login string `json:"login"`
}

type GHLabel struct {
	Name string `json:"name"`
}

type SyncIssuesOptions struct {
	State  string
	Limit  int
	OutDir string
}

func Run(args []string) error {
	if len(args) == 0 {
		PrintUsage()
		return nil
	}
	args = stripVersionArg(args)

	switch args[0] {
	case "help", "-h", "--help":
		PrintUsage()
		return nil
	case "install":
		return runInstall()
	case "issue", "issues":
		return runIssue(args[1:])
	case "pr":
		return runPR(args[1:])
	default:
		PrintUsage()
		return fmt.Errorf("unknown github command: %s", args[0])
	}
}

func PrintUsage() {
	fmt.Println("Usage: ./dialtone.sh github <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  issue sync [src_v1] [--state all|open|closed] [--limit N] [--out DIR]")
	fmt.Println("  issue list [src_v1] [--state all|open|closed] [--limit N]")
	fmt.Println("  issue view [src_v1] <issue-id>")
	fmt.Println("  pr [src_v1] [create|view|merge|close|review] [args]")
	fmt.Println("  test [src_v1]")
	fmt.Println("  install")
}

func runInstall() error {
	depsDir := getDialtoneEnv()
	ghBin := filepath.Join(depsDir, "gh", "bin", "gh")
	if _, err := os.Stat(ghBin); err == nil {
		logs.Info("GitHub CLI already installed at %s", ghBin)
		return nil
	}
	logs.Info("Installing GitHub CLI via core installer")
	cmd := exec.Command("./dialtone.sh", "install")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runIssue(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ./dialtone.sh github issue <sync|list|view> ...")
	}
	args = stripVersionArg(args)

	switch args[0] {
	case "sync":
		opts := SyncIssuesOptions{
			State:  "all",
			Limit:  500,
			OutDir: filepath.Join("plugins", "github", "src_v1", "issues"),
		}
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--state":
				if i+1 < len(args) {
					opts.State = args[i+1]
					i++
				}
			case "--limit":
				if i+1 < len(args) {
					n, err := strconv.Atoi(args[i+1])
					if err == nil && n > 0 {
						opts.Limit = n
					}
					i++
				}
			case "--out":
				if i+1 < len(args) {
					opts.OutDir = args[i+1]
					i++
				}
			}
		}
		count, err := SyncIssues(opts)
		if err != nil {
			return err
		}
		logs.Info("Synced %d issues to %s", count, opts.OutDir)
		return nil
	case "list":
		state := "open"
		limit := "30"
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--state":
				if i+1 < len(args) {
					state = args[i+1]
					i++
				}
			case "--limit":
				if i+1 < len(args) {
					limit = args[i+1]
					i++
				}
			}
		}
		gh := findGH()
		cmd := exec.Command(gh, "issue", "list", "--state", state, "--limit", limit)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "view":
		if len(args) < 2 {
			return fmt.Errorf("usage: ./dialtone.sh github issue view <issue-id>")
		}
		gh := findGH()
		cmd := exec.Command(gh, "issue", "view", args[1])
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unknown issue command: %s", args[0])
	}
}

func runPR(args []string) error {
	args = stripVersionArg(args)
	if len(args) == 0 {
		return prCreateOrUpdate("", "")
	}

	switch args[0] {
	case "create":
		return prCreateOrUpdate("", "")
	case "view":
		return runGHPassthrough(append([]string{"pr", "view"}, args[1:]...))
	case "merge":
		extra := args[1:]
		if len(extra) == 0 {
			extra = []string{"--merge", "--delete-branch"}
		}
		return runGHPassthrough(append([]string{"pr", "merge"}, extra...))
	case "close":
		return runGHPassthrough(append([]string{"pr", "close"}, args[1:]...))
	case "review":
		// Mark PR as ready for review.
		return runGHPassthrough(append([]string{"pr", "ready"}, args[1:]...))
	default:
		// `github pr "title" "body"` convenience form
		title := args[0]
		body := ""
		if len(args) > 1 {
			body = args[1]
		}
		return prCreateOrUpdate(title, body)
	}
}

func prCreateOrUpdate(title, body string) error {
	branch, err := currentBranch()
	if err != nil {
		return err
	}
	if branch == "main" || branch == "master" {
		return fmt.Errorf("cannot create PR from %s; create a feature branch first", branch)
	}
	if err := ensureBranchPushed(branch); err != nil {
		return err
	}

	gh := findGH()
	viewCmd := exec.Command(gh, "pr", "view", "--json", "number,url")
	viewOut, viewErr := viewCmd.Output()

	if viewErr == nil {
		logs.Info("PR already exists for branch %s", branch)
		if title != "" || body != "" {
			editArgs := []string{"pr", "edit"}
			if title != "" {
				editArgs = append(editArgs, "--title", title)
			}
			if body != "" {
				editArgs = append(editArgs, "--body", body)
			}
			if err := runGHPassthrough(editArgs); err != nil {
				return err
			}
		}
		fmt.Printf("%s\n", strings.TrimSpace(string(viewOut)))
		return nil
	}

	if title == "" {
		title = branch
	}
	if body == "" {
		body = fmt.Sprintf("Feature: %s", branch)
	}

	args := []string{"pr", "create", "--title", title, "--body", body}
	return runGHPassthrough(args)
}

func SyncIssues(opts SyncIssuesOptions) (int, error) {
	gh := findGH()
	if opts.State == "" {
		opts.State = "all"
	}
	if opts.Limit <= 0 {
		opts.Limit = 500
	}
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join("plugins", "github", "src_v1", "issues")
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return 0, err
	}

	cmd := exec.Command(
		gh,
		"issue", "list",
		"--state", opts.State,
		"--limit", strconv.Itoa(opts.Limit),
		"--json", "number,title,body,state,url,createdAt,updatedAt,author,labels",
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list issues: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return 0, fmt.Errorf("failed to parse issues: %w", err)
	}

	for _, issue := range issues {
		path := filepath.Join(opts.OutDir, fmt.Sprintf("%d.md", issue.Number))
		if err := os.WriteFile(path, []byte(RenderIssueTaskMarkdown(issue)), 0o644); err != nil {
			return 0, err
		}
	}
	return len(issues), nil
}

func RenderIssueTaskMarkdown(issue Issue) string {
	titleSlug := slug(issue.Title)
	if titleSlug == "" {
		titleSlug = fmt.Sprintf("issue-%d", issue.Number)
	}
	var b strings.Builder
	now := time.Now().UTC().Format(time.RFC3339)

	b.WriteString(fmt.Sprintf("# %d-%s\n", issue.Number, titleSlug))
	b.WriteString("### signature:\n")
	b.WriteString("- status: wait\n")
	b.WriteString(fmt.Sprintf("- issue: %d\n", issue.Number))
	b.WriteString("- source: github\n")
	if issue.URL != "" {
		b.WriteString(fmt.Sprintf("- url: %s\n", issue.URL))
	}
	b.WriteString(fmt.Sprintf("- synced-at: %s\n", now))
	b.WriteString("### description:\n")
	if strings.TrimSpace(issue.Body) == "" {
		b.WriteString("- TODO: fill from issue context\n")
	} else {
		for _, line := range strings.Split(strings.TrimSpace(issue.Body), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString("- " + line + "\n")
		}
	}
	b.WriteString("### tags:\n")
	if len(issue.Labels) == 0 {
		b.WriteString("- todo\n")
	} else {
		for _, l := range issue.Labels {
			if strings.TrimSpace(l.Name) != "" {
				b.WriteString("- " + strings.TrimSpace(l.Name) + "\n")
			}
		}
	}
	b.WriteString("### task-dependencies:\n")
	b.WriteString("### documentation:\n")
	b.WriteString("### test-condition-1:\n")
	b.WriteString("- TODO\n")
	b.WriteString("### test-command:\n")
	b.WriteString("- TODO\n")
	b.WriteString("### reviewed:\n")
	b.WriteString("### tested:\n")
	b.WriteString("### last-error-types:\n")
	b.WriteString("### last-error-times:\n")
	b.WriteString("### log-stream-command:\n")
	b.WriteString("- TODO\n")
	b.WriteString("### last-error-loglines:\n")
	b.WriteString("### notes:\n")
	b.WriteString(fmt.Sprintf("- title: %s\n", strings.TrimSpace(issue.Title)))
	b.WriteString(fmt.Sprintf("- state: %s\n", strings.TrimSpace(issue.State)))
	if strings.TrimSpace(issue.Author.Login) != "" {
		b.WriteString(fmt.Sprintf("- author: %s\n", strings.TrimSpace(issue.Author.Login)))
	}
	if strings.TrimSpace(issue.CreatedAt) != "" {
		b.WriteString(fmt.Sprintf("- created-at: %s\n", strings.TrimSpace(issue.CreatedAt)))
	}
	if strings.TrimSpace(issue.UpdatedAt) != "" {
		b.WriteString(fmt.Sprintf("- updated-at: %s\n", strings.TrimSpace(issue.UpdatedAt)))
	}
	return b.String()
}

func stripVersionArg(args []string) []string {
	if len(args) >= 2 && strings.HasPrefix(args[1], "src_v") {
		return append([]string{args[0]}, args[2:]...)
	}
	return args
}

func ensureBranchPushed(branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push branch %s: %w", branch, err)
	}
	return nil
}

func currentBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGHPassthrough(args []string) error {
	gh := findGH()
	cmd := exec.Command(gh, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func findGH() string {
	depsDir := getDialtoneEnv()
	ghPath := filepath.Join(depsDir, "gh", "bin", "gh")
	if _, err := os.Stat(ghPath); err == nil {
		return ghPath
	}
	if p, err := exec.LookPath("gh"); err == nil {
		return p
	}
	logs.Fatal("GitHub CLI ('gh') not found. Run './dialtone.sh github install'.")
	return ""
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	re := regexp.MustCompile(`[^a-z0-9\-]+`)
	s = re.ReplaceAllString(s, "")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}

func getDialtoneEnv() string {
	if env := strings.TrimSpace(os.Getenv("DIALTONE_ENV")); env != "" {
		if strings.HasPrefix(env, "~") {
			home, _ := os.UserHomeDir()
			env = filepath.Join(home, strings.TrimPrefix(env, "~"))
		}
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dialtone_env")
}
