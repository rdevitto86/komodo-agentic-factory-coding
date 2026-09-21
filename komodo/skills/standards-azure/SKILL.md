---
name: standards-azure
description: Conventions for engineering on Azure: subscriptions, naming, identity, and service choice.
globs: ["**/*.bicep", "**/main.bicep"]
---

# Azure

Provider-specific conventions for engineering on Azure. Principles and naming discipline live here; a specific VM size, region, service tier, or pricing figure never does — those drift, and the subscription's own portal, RBAC assignments, or the Azure docs are always the current source. Read the target subscription's existing role assignments and network layout before assuming a convention isn't already established there.

## IAM

- **Least privilege by default.** Start from zero role assignments and add only what the workload proves it needs. An assignment written by widening to Owner or Contributor is a defect, not a shortcut — grant the narrowest built-in role that covers the need, and reach for a custom role only when no built-in role fits.
- **Managed identities for workloads, users for humans.** A service, function, or VM authenticates via a system- or user-assigned managed identity; a person authenticates as a user (or federates through an identity provider) and is granted roles directly or through a group. Never provision a service principal secret for a workload when a managed identity can be used instead.
- **No hardcoded secrets or client secrets, ever** — not in source, config, IaC state, container layers, or CI logs. A client secret or connection string checked into a repo is a rotation event the moment it's found, not a lint warning.
- **Prefer managed identity and workload identity federation over static credentials.** Use workload identity federation for external workloads (CI/CD, other clouds) and system/user-assigned managed identities for anything running on Azure compute, rather than minting and distributing secrets or certificates.
- **Scope role assignments to resources or resource groups, not the subscription.** A subscription-level assignment is broader than intended almost every time; assign at the resource or resource-group scope, or with Azure AD Conditional Access / ABAC conditions where a narrower built-in scope isn't available.
- **Name custom roles and managed identities for what they grant, not who requested them** — a reviewer six months later should be able to tell an identity's blast radius from its name alone.
- **Separate human break-glass access from routine automation.** Emergency elevated access (e.g., a break-glass account with Privileged Identity Management activation) is logged, time-boxed, and distinct from the managed identities CI/CD or a running service uses day to day.
- **Review role assignments at every level of the management hierarchy.** Management-group- and subscription-level assignments inherit downward — a broad grant at the management-group or subscription level is as consequential as a resource-group-level one, and often less visible when auditing a single resource group.

## VNet and networking

- **Public-facing resources sit behind a load balancer, Application Gateway, or Azure Firewall; everything else stays on private addressing.** Compute, data stores, and internal services belong in subnets without public IPs, with outbound egress through NAT Gateway or Azure Firewall only where the workload needs it.
- **NSGs are the primary boundary; Azure Firewall is the coarse backstop.** A network security group is stateful and attached to a subnet or NIC — express the actual allowed-traffic intent there, scoped by port and source (an application security group or another NSG-scoped tag, not a wide CIDR, wherever the source is itself Azure-hosted). Reach for Azure Firewall or a hub-and-spoke firewall only for cross-VNet, egress-wide, or centrally audited rules that must hold even if a subnet's NSG is misconfigured.
- **No unrestricted inbound (`0.0.0.0/0`/`Any`) on anything but a public-facing load balancer, Application Gateway, or a resource meant to be internet-reachable.** Every other inbound rule names a specific source.
- **Subnet layout reflects the public/private split**, not the other way around — a subnet is only "public" because resources in it are assigned public IPs or sit behind a public frontend; get the NSG and route-table intent right and the subnet's role follows.
- **Cross-VNet and cross-subscription traffic uses VNet peering, a hub-and-spoke topology, or Private Link/Private Endpoint** — deliberately, not by routing through the public internet because it was easier to wire up.

## Service selection

Choose by workload shape, not by familiarity or what's already running elsewhere in the subscription. Read the Azure docs for the current names and tiers within a class — the principle below is what doesn't change.

- **Compute**: match the execution model to the workload. A short-lived, event-driven task fits a serverless/function model; a long-running or stateful process fits a container or managed-orchestration model; only a workload with a real OS-level or licensing constraint justifies a self-managed VM. Prefer the option with the least infrastructure to operate that still meets the workload's actual runtime and control requirements.
- **Storage**: match the access pattern. Unstructured objects and blobs fit an object store; a POSIX/SMB filesystem shared across compute fits a managed file service; block-level, single-attach storage fits a managed disk attached to one instance. Don't reach for a file share or a disk when an object store's access pattern would do.
- **Managed database over self-hosted**, by default. A managed relational or key-value/document service absorbs patching, failover, and backup that a self-run database on a VM would otherwise make the team's job. Choose relational vs. key-value/document vs. graph by query shape and consistency needs — see the database standard for the engine-agnostic rules once a class is chosen.
- **Every new resource gets a lifecycle owner and a cost/teardown plan before it's provisioned** — an orphaned resource is a security surface and a bill, not just clutter.
