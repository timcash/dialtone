# Ticket: cli-standardization
# Status: DONE

Standardize CLI structure across all core modules and plugins.

## SUBTASK: Create cli_review.md [x]
Audit all modules and plugins and document their current CLI state.

## SUBTASK: Setup Branch and Ticket System [x]
Create branch `cli-standardization` and initialize this ticket.

## SUBTASK: Standardize Core Modules [x]
- [x] Move build CLI to src/core/build/cli
- [x] Move install CLI to src/core/install/cli
- [x] Move ssh CLI to src/core/ssh/cli
- [x] Create basic CLI for browser, config, earth, logger, mock, util, web

## SUBTASK: Standardize Plugins [x]
- [x] Create CLI for jax-demo
- [x] Improve help support for diagnostic, vpn

## SUBTASK: Update Dev Entry point [x]
Update `src/dev.go` to use new CLI packages.

## SUBTASK: Verification [x]
Verify `help` and `--help` work for all modules.
