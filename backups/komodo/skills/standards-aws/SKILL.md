---
name: standards-aws
description: Conventions for engineering on AWS: accounts, naming, IAM discipline, and service choice.
globs: ["**/*.tf", "**/cdk/**", "**/infra/**", "**/infrastructure/**"]
---

# AWS

Provider-specific conventions for engineering on AWS. Principles and naming discipline live here; a specific instance type, region, service tier, or pricing figure never does — those drift, and the account's own console, IAM policy, or the AWS docs are always the current source. Read the target account's existing IAM policies and network layout before assuming a convention isn't already established there.

## IAM

- **Least privilege by default.** Start from zero permissions and add only what the workload proves it needs. A policy written by widening from `*` is a defect, not a shortcut.
- **Roles for workloads, users for humans.** A service, function, or instance assumes a role; a person authenticates as a user (or federates through an identity provider) and assumes a role for elevated or scoped work. Never issue long-lived credentials to a workload when a role can be assumed instead.
- **No hardcoded credentials, ever** — not in source, config, IaC state, container layers, or CI logs. An access key checked into a repo is a rotation event the moment it's found, not a lint warning.
- **Prefer temporary, assumed credentials** over static access keys. Where a workload must run outside AWS, federate through an identity provider rather than minting a long-lived key pair.
- **Scope policies to resources, not just actions.** An action-only allow (`s3:GetObject` on `*`) is broader than intended almost every time; attach a resource ARN or a condition key.
- **Name roles and policies for what they grant, not who requested them** — a reviewer six months later should be able to tell a role's blast radius from its name alone.
- **Separate human break-glass access from routine automation.** Emergency elevated access is logged, time-boxed, and distinct from the roles CI/CD or a running service assumes day to day.
- **Review trust policies as carefully as permission policies.** Who can assume a role is as consequential as what the role can do — an overly broad principal or a missing external-ID/condition check on cross-account trust is the same class of defect as an over-broad permission.

## VPC and networking

- **Public subnets hold only what must face the internet** — a load balancer, a NAT gateway, a bastion. Everything else — compute, data stores, internal services — belongs in a private subnet with outbound egress only where the workload needs it.
- **Security groups are the primary boundary; NACLs are the coarse backstop.** A security group is stateful and attached to the resource — express the actual allowed-traffic intent there, scoped by port and source (another security group, not a wide CIDR, wherever the source is itself AWS-hosted). Reach for a NACL only for subnet-wide, stateless rules — a coarse deny that must hold even if a security group is misconfigured.
- **No `0.0.0.0/0` ingress on anything but a public-facing load balancer or a resource meant to be internet-reachable.** Every other ingress rule names a specific source.
- **Route tables reflect the public/private split**, not the other way around — a subnet is only "public" because its route table sends `0.0.0.0/0` to an internet gateway; get the route table right and the subnet's role follows.
- **Cross-VPC and cross-account traffic uses peering, a transit gateway, or PrivateLink** — deliberately, not by routing through the public internet because it was easier to wire up.

## Service selection

Choose by workload shape, not by familiarity or what's already running elsewhere in the account. Read the AWS docs for the current names and tiers within a class — the principle below is what doesn't change.

- **Compute**: match the execution model to the workload. A short-lived, event-driven task fits a serverless/function model; a long-running or stateful process fits a container or managed-orchestration model; only a workload with a real OS-level or licensing constraint justifies a self-managed instance. Prefer the option with the least infrastructure to operate that still meets the workload's actual runtime and control requirements.
- **Storage**: match the access pattern. Unstructured objects and blobs fit an object store; a POSIX filesystem shared across compute fits a managed file service; block-level, single-attach storage fits a volume attached to one instance. Don't reach for a filesystem or a volume when an object store's access pattern would do.
- **Managed database over self-hosted**, by default. A managed relational or key-value/document service absorbs patching, failover, and backup that a self-run database on a compute instance would otherwise make the team's job. Choose relational vs. key-value/document vs. graph by query shape and consistency needs — see the database standard for the engine-agnostic rules once a class is chosen.
- **Every new resource gets a lifecycle owner and a cost/teardown plan before it's provisioned** — an orphaned resource is a security surface and a bill, not just clutter.
