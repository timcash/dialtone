can you get the current issues with the `./dialtone.sh github` plugin ? 

write them to prompt.md under the `# LLM:1` on line 4 using markdown
# LLM:1

### Current Issues with `./dialtone.sh github` Plugin

1.  **Resolved**: Critical build failure due to missing `nexttone` plugin has been fixed by purging `nexttone` references.
2.  **Resolved**: Hardcoded Vercel path and `check-deploy` logic have been removed from the GitHub plugin.
3.  **Resolved**: Inconsistent CLI naming has been fixed by standardizing on `./dialtone.sh` across the repo.
4.  **Resolved**: Deprecated `issue sync` command has been removed from the GitHub plugin.
5.  **Resolved**: Robust pre-flight checks for `gh` have been implemented. The plugin now prompts the user to run `./dialtone.sh github install` if the binary is missing.

# LLM:2 github test
Run `./dialtone.sh plugin test github` to verify the full GitHub PR lifecycle. The test creates a temporary branch, makes a commit, pushes to remote, creates a draft PR, marks it as ready, and then cleans up (closes PR and deletes remote/local branch).
