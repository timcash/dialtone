package test

import (
	"fmt"

	"dialtone/cli/src/dialtest"
)

func init() {
	dialtest.RegisterTicket("merge-main-guide")
	dialtest.AddSubtaskTest("install-tests", RunInstallTests, nil)
	dialtest.AddSubtaskTest("build-tests", RunBuildTests, nil)
	dialtest.AddSubtaskTest("rebase-build-branch", RunRebaseBuildBranch, nil)
	dialtest.AddSubtaskTest("merge-prs", RunMergePRs, nil)
}

func RunInstallTests() error {
	return fmt.Errorf("manual step: run './dialtone.sh --env test.env install test' on cli-standardization")
}

func RunBuildTests() error {
	return fmt.Errorf("manual step: run './dialtone.sh build test' on build-command-tests")
}

func RunRebaseBuildBranch() error {
	return fmt.Errorf("manual step: rebase build-command-tests on origin/main after merging PR 166")
}

func RunMergePRs() error {
	return fmt.Errorf("manual step: merge PRs 166 and 167 after tests pass")
}
