# Dialtone Marketplace Skill
Dialtone Marketplace provides a catalog for robot parts, services, and data offerings. It focuses on clear listings, service coordination, and reliable delivery.

## Core Focus
- Publish parts, assemblies, and service offerings.
- Coordinate field services and maintenance workflows.
- Provide data and AI service listings with clear terms.

## Capabilities
- Manage product and service catalog entries.
- Integrate ordering, fulfillment, and status updates.
- Support service scheduling and engineering requests.

## Inputs
- Listing metadata, pricing, and availability.
- Service requirements and scheduling constraints.
- Legal, compliance, and support details.

## Outputs
- Marketplace listings and update logs.
- Fulfillment status and service records.
- Analytics summaries for demand and usage.

## Workflows
This skill can use many workflows, including:
- `docs/workflows/ticket.md` for marketplace changes.
- `docs/workflows/subtask_expand.md` for listing flows.
- `docs/workflows/issue_review.md` for catalog issues.

## Plugins
This skill can use many plugins, including:
- `./dialtone.sh www` and `./dialtone.sh ui` for marketplace views.
- `./dialtone.sh logs` for operational tracking.
- `./dialtone.sh deploy` for content rollout.

## Example Tasks
- Add a new service category with pricing tiers.
- Build a fulfillment status dashboard.
- Update terms for a data services listing.

## Notes
- Keep terms and support coverage explicit.
- Track delivery SLAs and dependencies.
