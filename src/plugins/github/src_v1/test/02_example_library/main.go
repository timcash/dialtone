package main

import (
	"fmt"
	"strings"

	githubv1 "dialtone/dev/plugins/github/src_v1/go"
)

func main() {
	doc := githubv1.RenderIssueTaskMarkdown(githubv1.Issue{
		Number: 42,
		Title:  "Example issue for task conversion",
		Body:   "Implement minimal sync command",
		State:  "open",
		URL:    "https://github.com/example/repo/issues/42",
		Labels: []githubv1.GHLabel{{Name: "automation"}},
	}, githubv1.RenderOptions{})
	if !strings.Contains(doc, "- status: wait") {
		panic("missing wait status")
	}
	fmt.Println("GITHUB_LIBRARY_EXAMPLE_PASS")
}
