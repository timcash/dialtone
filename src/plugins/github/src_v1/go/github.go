package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	logs "dialtone/dev/plugins/logs/src_v1/go"
)

type Issue struct {
	Number    int         `json:"number"`
	Title     string      `json:"title"`
	Body      string      `json:"body"`
	State     string      `json:"state"`
	URL       string      `json:"url"`
	CreatedAt string      `json:"createdAt"`
	UpdatedAt string      `json:"updatedAt"`
	Author    GHAuthor    `json:"author"`
	Labels    []GHLabel   `json:"labels"`
	Comments  []GHComment `json:"comments"`
}

type GHAuthor struct {
	Login string `json:"login"`
}

type GHLabel struct {
	Name string `json:"name"`
}

type GHComment struct {
	Body      string   `json:"body"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	Author    GHAuthor `json:"author"`
}

type SyncIssuesOptions struct {
	State  string
	Limit  int
	OutDir string
}

type PushIssuesOptions struct {
	OutDir string
	Force  bool
}

type RenderOptions struct {
	SyncMeta         SyncMeta
	CommentsGitHub   []string
	CommentsOutbound []string
}

type SyncMeta struct {
	GitHubUpdatedAt  string
	LastPulledAt     string
	LastPushedAt     string
	GitHubLabelsHash string
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
	fmt.Println("  issue sync [src_v1] [--state all|open|closed] [--limit N] [--out DIR]   # default state=open")
	fmt.Println("  issue push [src_v1] [--out DIR] [--force]")
	fmt.Println("  issue delete-closed [src_v1] [--out DIR] [--limit N] [--dry-run]")
	fmt.Println("  issue print [src_v1] <issue-id> [--out DIR]")
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
		return fmt.Errorf("usage: ./dialtone.sh github issue <sync|push|delete-closed|print|list|view> ...")
	}
	args = stripVersionArg(args)

	switch args[0] {
	case "sync":
		opts := SyncIssuesOptions{
			State:  "open",
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
	case "delete-closed":
		outDir := filepath.Join("plugins", "github", "src_v1", "issues")
		limit := 500
		dryRun := false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--out":
				if i+1 < len(args) {
					outDir = args[i+1]
					i++
				}
			case "--limit":
				if i+1 < len(args) {
					n, err := strconv.Atoi(args[i+1])
					if err == nil && n > 0 {
						limit = n
					}
					i++
				}
			case "--dry-run":
				dryRun = true
			}
		}
		deleted, missing, err := DeleteClosedIssueFiles(outDir, limit, dryRun)
		if err != nil {
			return err
		}
		if dryRun {
			logs.Info("Dry-run complete: would delete=%d already-missing=%d (dir=%s)", deleted, missing, outDir)
		} else {
			logs.Info("Deleted closed issue files: deleted=%d already-missing=%d (dir=%s)", deleted, missing, outDir)
		}
		return nil
	case "print":
		if len(args) < 2 {
			return fmt.Errorf("usage: ./dialtone.sh github issue print <issue-id> [--out DIR]")
		}
		outDir := filepath.Join("plugins", "github", "src_v1", "issues")
		issueID := strings.TrimSpace(args[1])
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--out":
				if i+1 < len(args) {
					outDir = args[i+1]
					i++
				}
			}
		}
		return PrintIssue(issueID, outDir)
	case "push":
		opts := PushIssuesOptions{OutDir: filepath.Join("plugins", "github", "src_v1", "issues")}
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--out":
				if i+1 < len(args) {
					opts.OutDir = args[i+1]
					i++
				}
			case "--force":
				opts.Force = true
			}
		}
		sent, skipped, err := PushIssues(opts)
		if err != nil {
			return err
		}
		logs.Info("Issue push complete: sent=%d skipped=%d", sent, skipped)
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

func PrintIssue(issueID, outDir string) error {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return errors.New("missing issue id")
	}
	if outDir == "" {
		outDir = filepath.Join("plugins", "github", "src_v1", "issues")
	}
	path := filepath.Join(outDir, issueID+".md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed reading issue file %s: %w", path, err)
	}

	text := string(raw)
	sections := parseSections(text)

	title := issueTitleFromMarkdown(text, sections["notes"], issueID)
	status := valueFromBullet(sections["signature"], "status")
	url := valueFromBullet(sections["signature"], "url")
	ghUpdated := valueFromBullet(sections["sync"], "github-updated-at")
	lastPulled := valueFromBullet(sections["sync"], "last-pulled-at")
	lastPushed := valueFromBullet(sections["sync"], "last-pushed-at")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Issue %s: %s\n\n", issueID, title))
	if status != "" {
		b.WriteString(fmt.Sprintf("- Status: `%s`\n", status))
	}
	if url != "" {
		b.WriteString(fmt.Sprintf("- URL: %s\n", url))
	}
	if ghUpdated != "" {
		b.WriteString(fmt.Sprintf("- GitHub Updated: `%s`\n", ghUpdated))
	}
	if lastPulled != "" {
		b.WriteString(fmt.Sprintf("- Last Pulled: `%s`\n", lastPulled))
	}
	if lastPushed != "" {
		b.WriteString(fmt.Sprintf("- Last Pushed: `%s`\n", lastPushed))
	}

	writeSection := func(name string, lines []string) {
		if len(lines) == 0 || allBlank(lines) {
			return
		}
		b.WriteString("\n")
		b.WriteString("## " + name + "\n")
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			b.WriteString(l + "\n")
		}
	}

	writeSection("Description", sections["description"])
	writeSection("Tags", sections["tags"])
	writeSection("Comments (GitHub)", sections["comments-github"])
	writeSection("Comments (Outbound)", sections["comments-outbound"])
	writeSection("Notes", sections["notes"])

	fmt.Print(strings.TrimRight(b.String(), "\n") + "\n")
	return nil
}

func issueTitleFromMarkdown(raw string, notes []string, fallback string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	if t := valueFromBullet(notes, "title"); t != "" {
		return t
	}
	return "issue-" + fallback
}

func valueFromBullet(lines []string, key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		parts := strings.SplitN(payload, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(parts[0]))
		v := strings.TrimSpace(parts[1])
		if k == key {
			return v
		}
	}
	return ""
}

func allBlank(lines []string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			return false
		}
	}
	return true
}

func DeleteClosedIssueFiles(outDir string, limit int, dryRun bool) (int, int, error) {
	gh := findGH()
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join("plugins", "github", "src_v1", "issues")
	}
	if limit <= 0 {
		limit = 500
	}
	cmd := exec.Command(
		gh,
		"issue", "list",
		"--state", "closed",
		"--limit", strconv.Itoa(limit),
		"--json", "number",
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list closed issues: %w", err)
	}
	var closed []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(output, &closed); err != nil {
		return 0, 0, fmt.Errorf("failed to parse closed issues: %w", err)
	}

	deleted := 0
	missing := 0
	for _, item := range closed {
		path := filepath.Join(outDir, fmt.Sprintf("%d.md", item.Number))
		_, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				missing++
				continue
			}
			return deleted, missing, statErr
		}
		if dryRun {
			deleted++
			continue
		}
		if err := os.Remove(path); err != nil {
			return deleted, missing, err
		}
		deleted++
	}
	return deleted, missing, nil
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
		return runGHPassthrough(append([]string{"pr", "ready"}, args[1:]...))
	default:
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
		"--json", "number,title,body,state,url,createdAt,updatedAt,author,labels,comments",
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to list issues: %w", err)
	}

	var issues []Issue
	if err := json.Unmarshal(output, &issues); err != nil {
		return 0, fmt.Errorf("failed to parse issues: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, issue := range issues {
		path := filepath.Join(opts.OutDir, fmt.Sprintf("%d.md", issue.Number))
		existingRaw, _ := os.ReadFile(path)
		existingSections := parseSections(string(existingRaw))
		existingSync := parseSyncMeta(existingSections["sync"])
		outbound := normalizeOutbound(existingSections["comments-outbound"])
		render := RenderOptions{
			SyncMeta: SyncMeta{
				GitHubUpdatedAt:  issue.UpdatedAt,
				LastPulledAt:     now,
				LastPushedAt:     existingSync.LastPushedAt,
				GitHubLabelsHash: labelsHash(issue.Labels),
			},
			CommentsGitHub:   formatGitHubComments(issue.Comments),
			CommentsOutbound: outbound,
		}
		if err := os.WriteFile(path, []byte(RenderIssueTaskMarkdown(issue, render)), 0o644); err != nil {
			return 0, err
		}
	}
	return len(issues), nil
}

func PushIssues(opts PushIssuesOptions) (int, int, error) {
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join("plugins", "github", "src_v1", "issues")
	}
	files, err := filepath.Glob(filepath.Join(opts.OutDir, "*.md"))
	if err != nil {
		return 0, 0, err
	}
	sort.Strings(files)

	now := time.Now().UTC().Format(time.RFC3339)
	sent := 0
	skipped := 0
	for _, path := range files {
		rawBytes, err := os.ReadFile(path)
		if err != nil {
			return sent, skipped, err
		}
		raw := string(rawBytes)
		sections := parseSections(raw)

		issueID, err := detectIssueID(path, sections["signature"])
		if err != nil {
			logs.Warn("Skipping %s: %v", path, err)
			skipped++
			continue
		}

		live, err := fetchIssueByNumber(issueID)
		if err != nil {
			logs.Warn("Skipping #%d (%s): failed to fetch live issue: %v", issueID, path, err)
			skipped++
			continue
		}

		syncMeta := parseSyncMeta(sections["sync"])
		if needsConflictWarning(syncMeta.GitHubUpdatedAt, live.UpdatedAt) && !opts.Force {
			logs.Warn("Skipping #%d (%s): GitHub updated at %s (local known %s). Run issue sync first or use --force.", issueID, path, live.UpdatedAt, syncMeta.GitHubUpdatedAt)
			skipped++
			continue
		}

		pending, idxs := pendingOutboundComments(sections["comments-outbound"])
		if len(pending) == 0 {
			continue
		}

		postFailed := false
		for _, c := range pending {
			if err := postIssueComment(issueID, c); err != nil {
				logs.Warn("Skipping #%d comment push due to error: %v", issueID, err)
				skipped++
				postFailed = true
				break
			}
			sent++
		}
		if postFailed {
			continue
		}

		sections["comments-outbound"] = markOutboundSent(sections["comments-outbound"], idxs, now)
		updatedLive, err := fetchIssueByNumber(issueID)
		if err == nil {
			sections["comments-github"] = formatGitHubComments(updatedLive.Comments)
			syncMeta.GitHubUpdatedAt = updatedLive.UpdatedAt
			syncMeta.GitHubLabelsHash = labelsHash(updatedLive.Labels)
		}
		syncMeta.LastPushedAt = now
		if syncMeta.LastPulledAt == "" {
			syncMeta.LastPulledAt = now
		}
		sections["sync"] = formatSyncMeta(syncMeta)

		if err := os.WriteFile(path, []byte(rebuildMarkdown(raw, sections)), 0o644); err != nil {
			return sent, skipped, err
		}

	}

	return sent, skipped, nil
}

func RenderIssueTaskMarkdown(issue Issue, opts RenderOptions) string {
	titleSlug := slug(issue.Title)
	if titleSlug == "" {
		titleSlug = fmt.Sprintf("issue-%d", issue.Number)
	}
	var b strings.Builder
	now := time.Now().UTC().Format(time.RFC3339)

	if opts.SyncMeta.LastPulledAt == "" {
		opts.SyncMeta.LastPulledAt = now
	}
	if opts.SyncMeta.GitHubUpdatedAt == "" {
		opts.SyncMeta.GitHubUpdatedAt = issue.UpdatedAt
	}
	if opts.SyncMeta.GitHubLabelsHash == "" {
		opts.SyncMeta.GitHubLabelsHash = labelsHash(issue.Labels)
	}
	if len(opts.CommentsGitHub) == 0 {
		opts.CommentsGitHub = formatGitHubComments(issue.Comments)
	}
	if len(opts.CommentsOutbound) == 0 {
		opts.CommentsOutbound = normalizeOutbound(nil)
	}

	b.WriteString(fmt.Sprintf("# %d-%s\n", issue.Number, titleSlug))
	b.WriteString("### signature:\n")
	b.WriteString("- status: wait\n")
	b.WriteString(fmt.Sprintf("- issue: %d\n", issue.Number))
	b.WriteString("- source: github\n")
	if issue.URL != "" {
		b.WriteString(fmt.Sprintf("- url: %s\n", issue.URL))
	}
	b.WriteString(fmt.Sprintf("- synced-at: %s\n", now))
	b.WriteString("### sync:\n")
	for _, line := range formatSyncMeta(opts.SyncMeta) {
		b.WriteString(line + "\n")
	}
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
	b.WriteString("### comments-github:\n")
	for _, line := range opts.CommentsGitHub {
		b.WriteString(line + "\n")
	}
	b.WriteString("### comments-outbound:\n")
	for _, line := range opts.CommentsOutbound {
		b.WriteString(line + "\n")
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

func formatGitHubComments(comments []GHComment) []string {
	if len(comments) == 0 {
		return []string{"- none"}
	}
	out := make([]string, 0, len(comments))
	for _, c := range comments {
		body := strings.TrimSpace(strings.ReplaceAll(c.Body, "\n", " "))
		if body == "" {
			body = "(empty)"
		}
		if len(body) > 280 {
			body = body[:280] + "..."
		}
		author := strings.TrimSpace(c.Author.Login)
		if author == "" {
			author = "unknown"
		}
		created := strings.TrimSpace(c.CreatedAt)
		if created == "" {
			created = strings.TrimSpace(c.UpdatedAt)
		}
		if created == "" {
			created = "unknown-time"
		}
		out = append(out, fmt.Sprintf("- [%s] @%s: %s", created, author, body))
	}
	return out
}

func normalizeOutbound(lines []string) []string {
	clean := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			clean = append(clean, line)
		}
	}
	if len(clean) == 0 {
		return []string{"- TODO: add a bullet comment here to post to GitHub"}
	}
	return clean
}

func parseSections(raw string) map[string][]string {
	sections := map[string][]string{}
	if strings.TrimSpace(raw) == "" {
		return sections
	}
	lines := strings.Split(raw, "\n")
	current := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "### ") && strings.HasSuffix(line, ":") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "### "), ":")
			current = strings.TrimSpace(name)
			if _, ok := sections[current]; !ok {
				sections[current] = []string{}
			}
			continue
		}
		if current != "" {
			sections[current] = append(sections[current], line)
		}
	}
	return sections
}

func rebuildMarkdown(original string, updates map[string][]string) string {
	lines := strings.Split(original, "\n")
	var out []string
	i := 0
	seen := map[string]bool{}
	for i < len(lines) {
		line := lines[i]
		if strings.HasPrefix(line, "### ") && strings.HasSuffix(line, ":") {
			name := strings.TrimSuffix(strings.TrimPrefix(line, "### "), ":")
			name = strings.TrimSpace(name)
			out = append(out, line)
			if replacement, ok := updates[name]; ok {
				seen[name] = true
				for _, repl := range replacement {
					out = append(out, repl)
				}
				i++
				for i < len(lines) {
					next := lines[i]
					if strings.HasPrefix(next, "### ") && strings.HasSuffix(next, ":") {
						break
					}
					i++
				}
				continue
			}
		}
		out = append(out, line)
		i++
	}

	for _, name := range []string{"sync", "comments-github", "comments-outbound"} {
		if seen[name] {
			continue
		}
		replacement, ok := updates[name]
		if !ok {
			continue
		}
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, fmt.Sprintf("### %s:", name))
		out = append(out, replacement...)
	}

	res := strings.Join(out, "\n")
	if !strings.HasSuffix(res, "\n") {
		res += "\n"
	}
	return res
}

func parseSyncMeta(lines []string) SyncMeta {
	meta := SyncMeta{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		switch k {
		case "github-updated-at":
			meta.GitHubUpdatedAt = v
		case "last-pulled-at":
			meta.LastPulledAt = v
		case "last-pushed-at":
			meta.LastPushedAt = v
		case "github-labels-hash":
			meta.GitHubLabelsHash = v
		}
	}
	return meta
}

func formatSyncMeta(meta SyncMeta) []string {
	return []string{
		"- github-updated-at: " + strings.TrimSpace(meta.GitHubUpdatedAt),
		"- last-pulled-at: " + strings.TrimSpace(meta.LastPulledAt),
		"- last-pushed-at: " + strings.TrimSpace(meta.LastPushedAt),
		"- github-labels-hash: " + strings.TrimSpace(meta.GitHubLabelsHash),
	}
}

func pendingOutboundComments(lines []string) ([]string, []int) {
	pending := []string{}
	idxs := []int{}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if body == "" || strings.HasPrefix(body, "TODO:") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(body), "[sent") {
			continue
		}
		pending = append(pending, body)
		idxs = append(idxs, i)
	}
	return pending, idxs
}

func markOutboundSent(lines []string, idxs []int, when string) []string {
	out := append([]string{}, lines...)
	idxSet := map[int]struct{}{}
	for _, i := range idxs {
		idxSet[i] = struct{}{}
	}
	for i := range out {
		if _, ok := idxSet[i]; !ok {
			continue
		}
		line := strings.TrimSpace(out[i])
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		out[i] = fmt.Sprintf("- [sent %s] %s", when, body)
	}
	return out
}

func postIssueComment(issueID int, body string) error {
	gh := findGH()
	cmd := exec.Command(gh, "issue", "comment", strconv.Itoa(issueID), "--body", body)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fetchIssueByNumber(issueID int) (Issue, error) {
	gh := findGH()
	cmd := exec.Command(gh, "issue", "view", strconv.Itoa(issueID), "--json", "number,title,body,state,url,createdAt,updatedAt,author,labels,comments")
	out, err := cmd.Output()
	if err != nil {
		return Issue{}, err
	}
	var issue Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func detectIssueID(path string, signature []string) (int, error) {
	for _, line := range signature {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- issue:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "- issue:"))
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n, nil
		}
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	n, err := strconv.Atoi(base)
	if err == nil && n > 0 {
		return n, nil
	}
	return 0, fmt.Errorf("cannot detect issue id")
}

func needsConflictWarning(known, live string) bool {
	known = strings.TrimSpace(known)
	live = strings.TrimSpace(live)
	if known == "" || live == "" {
		return false
	}
	knownTime, errKnown := time.Parse(time.RFC3339, known)
	liveTime, errLive := time.Parse(time.RFC3339, live)
	if errKnown != nil || errLive != nil {
		return known != live
	}
	return liveTime.After(knownTime)
}

func labelsHash(labels []GHLabel) string {
	vals := make([]string, 0, len(labels))
	for _, l := range labels {
		name := strings.TrimSpace(strings.ToLower(l.Name))
		if name != "" {
			vals = append(vals, name)
		}
	}
	sort.Strings(vals)
	return strings.Join(vals, ",")
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
