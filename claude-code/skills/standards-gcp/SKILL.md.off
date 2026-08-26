---
name: standards-gcp
description: GCP conventions — IAM least privilege, VPC/networking basics, service-selection principles. Load before writing or reviewing IAM roles/service accounts, VPC/subnet/firewall-rule config, or choosing which GCP service class to use.
user-invocable: false
---

# GCP

Provider-specific conventions for engineering on GCP. Principles and naming discipline live here; a specific machine type, region, service tier, or pricing figure never does — those drift, and the project's own console, IAM policy, or the GCP docs are always the current source. Read the target project's existing IAM bindings and network layout before assuming a convention isn't already established there.

## IAM

- **Least privilege by default.** Start from zero roles and add only what the workload proves it needs. A binding written by widening from a basic role (Owner/Editor) is a defect, not a shortcut — grant the narrowest predefined role that covers the need, and reach for a custom role only when no predefined role fits.
- **Service accounts for workloads, users for humans.** A service, function, or instance authenticates as a service account; a person authenticates as a user (or federates through an identity provider) and is granted roles directly or through a group. Never mint a service account key for a workload when workload identity federation or attached-identity auth can be used instead.
- **No exported service account keys, ever** — not in source, config, IaC state, container layers, or CI logs. A downloaded JSON key checked into a repo is a rotation event the moment it's found, not a lint warning.
- **Prefer workload identity over static keys.** Use workload identity federation for external workloads (CI/CD, other clouds) and attached service accounts for anything running on GCP compute, rather than minting and distributing key files.
- **Scope bindings to resources, not just projects.** A project-level role grant is broader than intended almost every time; bind at the resource, folder boundary, or with IAM Conditions where the resource-level grant isn't available.
- **Name custom roles and service accounts for what they grant, not who requested them** — a reviewer six months later should be able to tell a service account's blast radius from its name alone.
- **Separate human break-glass access from routine automation.** Emergency elevated access is logged, time-boxed, and distinct from the service accounts CI/CD or a running service uses day to day.
- **Review IAM policy bindings at every level of the resource hierarchy.** Organization- and folder-level bindings inherit downward — a broad grant at the org or folder level is as consequential as a project-level one, and often less visible when auditing a single project.

## VPC and networking

- **Public-facing resources sit behind a load balancer or Cloud NAT; everything else stays without an external IP.** Compute, data stores, and internal services belong on internal-only addresses with egress through Cloud NAT or a proxy where outbound access is needed.
- **Firewall rules are the primary boundary, scoped by tag or service account, not by wide CIDR.** Express the actual allowed-traffic intent with target tags or service accounts and a specific source (another tag/service account, not `0.0.0.0/0`, wherever the source is itself GCP-hosted). VPC Firewall rules are the GCP analogue of a security group: attach intent to the resource, not the subnet.
- **No `0.0.0.0/0` ingress on anything but a public-facing load balancer or a resource meant to be internet-reachable.** Every other ingress rule names a specific source range, tag, or service account.
- **Subnet mode and routes reflect the public/private split**, not the other way around — a subnet is only "public" because its resources are assigned external IPs or sit behind a forwarding rule; get the firewall and route intent right and the subnet's role follows. Prefer custom-mode VPCs over the auto-mode default so subnet ranges are deliberate.
- **Cross-VPC and cross-project traffic uses VPC peering, Shared VPC, or Private Service Connect** — deliberately, not by routing through the public internet because it was easier to wire up.

## Service selection

Choose by workload shape, not by familiarity or what's already running elsewhere in the project. Read the GCP docs for the current names and tiers within a class — the principle below is what doesn't change.

- **Compute**: match the execution model to the workload. A short-lived, event-driven task fits a serverless/function model; a long-running or stateful process fits a container or managed-orchestration model; only a workload with a real OS-level or licensing constraint justifies a self-managed VM. Prefer the option with the least infrastructure to operate that still meets the workload's actual runtime and control requirements.
- **Storage**: match the access pattern. Unstructured objects and blobs fit an object store; a POSIX filesystem shared across compute fits a managed file service; block-level, single-attach storage fits a persistent disk attached to one instance. Don't reach for a filesystem or a disk when an object store's access pattern would do.
- **Managed database over self-hosted**, by default. A managed relational or key-value/document service absorbs patching, failover, and backup that a self-run database on a VM would otherwise make the team's job. Choose relational vs. key-value/document vs. graph by query shape and consistency needs — see `standards-database` for the engine-agnostic rules once a class is chosen.
- **Every new resource gets a lifecycle owner and a cost/teardown plan before it's provisioned** — an orphaned resource is a security surface and a bill, not just clutter.

## Reference

Treat the Google Cloud Architecture Framework's Security, Reliability, and Cost Optimization pillars as the depth reference when a decision here needs a defensible justification beyond the principle stated. Read the current pillar text rather than relying on a summarized version — the pillars themselves are revised over time.
