### improve template
- integrate dag visualization
- update smoke with install, lint, build, dev, sections into the SMOKE.md
- update ui styles
- add a DIALTONE> xterm.js section to the template

### integrate nix as the package manager
- show a full ./dialtone.sh workflow as a starting point
- start with install from just the `dialtone.sh` or `dialtone.ps1` wrapper
- get nix installed via the `./dialtone.sh nix install` command
- show a plugin install via `./dialtone.sh plugin install <plugin-name>`

### add key management tools
- allow USER> and LLM-*> to send a password and get authed with DIALTONE>

### build DIALTONE> cli 
- start with a simple dailtone.sh scripted interatcion
- update core with the new dialtone process manager to spawn subtones
- log all interactions in swarm autolog
- integrate with swarm for the dag task database
- show a workflow of starting on a new computer for the first time and getting things installed 
