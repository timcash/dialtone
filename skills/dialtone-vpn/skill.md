# Dialtone VPN Skill
Dialtone VPN provides secure peer discovery and private networking for users and robots. It prioritizes stable connectivity, identity management, and access control.

## Core Focus
- Establish private connectivity across robots and operators.
- Manage identities and access control lists (ACLs).
- Support peer discovery and secure routing.

## Capabilities
- Assign unique identities to users and robots.
- Enforce ACL policies for authorized access.
- Maintain resilient links across NAT and varied networks.

## Inputs
- Peer identities and ACL policies.
- Network topology and routing constraints.
- Deployment environment and security requirements.

## Outputs
- Configured VPN profiles and ACL rules.
- Connection status and diagnostics.
- Security recommendations and audit notes.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for VPN features.
- `docs/workflows/issue_review.md` for connectivity bugs.
- `docs/workflows/subtask_expand.md` for ACL changes.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh vpn` for network management.
- `./dialtone.sh diagnostic` and `./dialtone.sh logs` for visibility.
- `./dialtone.sh deploy` for rollout and updates.

## Example Tasks
- Add a new ACL rule set for a robot fleet.
- Diagnose peer discovery failures on a subnet.
- Harden VPN configuration for a production deployment.

## Notes
- Prefer least-privilege ACLs and explicit allow rules.
- Document identity requirements for operators.
