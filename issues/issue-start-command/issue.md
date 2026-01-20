# Issue: Implement `issue start` command in `dialtone-dev`

**Branch:** `issue/issue-start-command`  
**Tags:** tag:feature tag:priority tag:cli

## 🎯 Goal
Implement a command that takes a human-written issue markdown file from the `./issues` folder and bootstraps the developer environment. 

The command `dialtone-dev issue start <issue-name>` should:
1. Parse the issue file for the `Branch` metadata.
2. Automatically create a new git branch.
3. Create a standardized folder and file structure for the task, especially if it involves a new plugin.

## 📋 Task List

- [ ] **CLI Implementation**:
    - [ ] Modify `src/dev.go` to add the `issue start` subcommand.
    - [ ] Implement logic to read and parse `./issues/<issue-name>.md`.
- [ ] **Git Automation**:
    - [ ] Execute `git checkout -b <branch-name>` automatically after parsing.
- [ ] **Scaffolding Logic**:
    - [ ] If the issue identifies a new plugin, create the following structure:
        - [ ] `./issues/<issue-name>/plan.md` (copied/initialized from the issue description).
        - [ ] `./src/plugins/<plugin_name>/app/`
        - [ ] `./src/plugins/<plugin_name>/cli/`
        - [ ] `./src/plugins/<plugin_name>/tests/`
- [ ] **Verification**:
    - [ ] Run `go run dialtone-dev.go build --local`.
    - [ ] Test the command: `go run dialtone-dev.go issue start issue-start-command`.
    - [ ] Verify branch is created and directories exist.

## 📝 Collaborative Notes
- The parser should be resilient to whitespace in the metadata section.
- If the branch already exists, it should probably just switch to it or warn the user.
- The "autocoder" agent will rely on this scaffolding to find where to start writing code (e.g., looking at `./issues/<issue-name>/plan.md`).

---
*Template version: 1.0. To start work: `dialtone-dev issue start issue-start-command`*