# Branch: cloudflare-tunnel
# Tags: enhancement, ready

# Goal
Integrate Cloudflare Tunnels as a Dialtone plugin (`cloudflare`) to enable secure remote access and local service forwarding. This involves wrapping the `cloudflared` CLI and providing a streamlined experience for exposing local HTTP servers to the web.

## SUBTASK: Create the Cloudflare plugin scaffold
- name: cloudflare-plugin-add
- description: Create the plugin structure in `src/plugins/cloudflare` using the CLI.
- test-description: Verify directory exists and has a README.md.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-plugin-add`
- status: done

## SUBTASK: Implement Cloudflare installation logic
- name: cloudflare-install
- description: Implement `install.go` in the cloudflare plugin to download and verify the `cloudflared` binary for the current platform.
- test-description: Verify `cloudflared` is executable after running install.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-install`
- status: done

## SUBTASK: Implement Cloudflare Login
- name: cloudflare-login
- description: Add a `login` subcommand to the cloudflare plugin that wraps `cloudflared tunnel login`.
- test-description: Verify the command triggers the cloudflared login process.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-login`
- status: done

## SUBTASK: Implement Tunnel Management
- name: cloudflare-tunnel-mgmt
- description: Implement `tunnel create` and `tunnel list` subcommands to manage named tunnels.
- test-description: Verify commands successfully call wrapped `cloudflared` logic.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-tunnel-mgmt`
- status: done

## SUBTASK: Implement Serve/Forwarding Logic
- name: cloudflare-serve
- description: Implement a `serve` command (e.g., `./dialtone.sh cloudflare serve <port> [--tunnel <name>]`) to forward local HTTP traffic.
- test-description: Verify the command starts a tunnel session.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-serve`
- status: done

## SUBTASK: start ticket work via `dialtone.sh` cli
- name: ticket-start
- description: run the cli command `dialtone.sh ticket start cloudflare-tunnel`
- test-description: verify ticket is scaffolded
- test-command: `./dialtone.sh plugin test <plugin-name>`
- status: done

## SUBTASK: Use DIALTONE_HOSTNAME as Cloudflare subdomain
- name: cloudflare-hostname-subdomain
- description: Update the Cloudflare plugin to use the `DIALTONE_HOSTNAME` environment variable as the default subdomain when routing or serving. This should facilitate configuration-free routing for nodes like `<DIALTONE_HOSTNAME>.dialtone.earth`.
- test-description: Verify that the `cloudflare` plugin logic correctly retrieves `DIALTONE_HOSTNAME` and uses it in the `tunnel route dns` logic.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-hostname-subdomain`
- status: done

## SUBTASK: Implement Tunnel Cleanup
- name: cloudflare-tunnel-cleanup
- description: Add a `cleanup` subcommand to `dialtone cloudflare tunnel` that identifies and terminates all locally running `cloudflared` processes. This ensures no orphaned tunnels occupy ports or consume resources.
- test-description: Verify that running the cleanup command successfully terminates any background `cloudflared` processes.
- test-command: `./dialtone.sh plugin test <plugin-name> --subtask cloudflare-tunnel-cleanup`
- status: done

## SUBTASK: complete ticket via `dialtone.sh` cli
- name: ticket-done
- description: run the ticket cli to verify all steps to complete the ticket
- test-description: validates all ticket subtasks are done
- test-command: `dialtone.sh ticket done cloudflare-tunnel`
- status: done

## Collaborative Notes
- **Context**: [src/plugins/cloudflare](file:///Users/tim/code/dialtone/src/plugins/cloudflare)
- **Implementation Notes**: 
  - We should store the `cloudflared` binary in the plugin's `bin/` directory or use the system path if available.
  - The `serve` command should ideally handle both anonymous "TryCloudflare" tunnels and authenticated named tunnels.
- **Reference**: Cloudflare Tunnel Documentation: https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/
