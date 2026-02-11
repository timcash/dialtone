# Gemini CLI Agent Guide

## Platform-Specific Execution

This project provides a unified entry point for various tasks, but the command depends on your operating system:

### Windows
Use the PowerShell script or the command wrapper:
```powershell
.\dialtone <command> [options]
```
*Note: The agent should always use `.\dialtone` on Windows.*

### Linux and macOS
Use the shell script:
```bash
./dialtone.sh <command> [options]
```

## Key Commands
- `.\dialtone wsl smoke src_v1`: Runs the WSL smoke test suite.
- `.\dialtone install`: Installs necessary dependencies.
- `.\dialtone build`: Builds the project.
