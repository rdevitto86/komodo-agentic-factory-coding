---
name: terraform
description: Terraform and AWS — module layout, state, tagging, Fargate service shape. Load before reading or writing any .tf, .tfvars, or .hcl file.
user-invocable: false
---

# Terraform

Zero comments, including `# TODO` markers in generated stubs.

## Module layout

A module is `main.tf`, `variables.tf`, `outputs.tf`. Nothing else at the root unless it earns its place.

- **Every variable has a type and a description field.** A variable with no default is required — do not fake a default to avoid the prompt.
- **Every output has a description.** Output only what a caller actually consumes.
- **Pin provider versions** in `required_providers`. Pin module sources to a tag, never a branch.
- **No hardcoded account IDs, ARNs, or regions.** They come from variables or data sources.

## State

- **Remote state with locking.** S3 plus DynamoDB, or the equivalent.
- **One state file per environment.** Never share state across environments.
- **Never edit state by hand.** `terraform state mv` and `import` are the supported paths.
- **`terraform plan` output is the review artefact.** An apply without a reviewed plan is not a change, it is an incident.

## Tagging

Every taggable resource carries at minimum:

| Tag | Value |
|---|---|
| `env` | the environment name |
| `app_name` | the service this belongs to |
| `managed_by` | `terraform` |

Set these once in a `default_tags` provider block rather than repeating them per resource.

## Fargate service module

A service module produces this component set. Missing any one of them is the usual cause of a service that deploys but does not work:

- **Task definition** — image, CPU/memory, environment, secrets references
- **ECS service** — load balancer attachment and service discovery registration
- **CloudWatch log group** — with an explicit retention period, never the default of never-expire
- **Task execution role** — scoped to the secrets and log group this service actually uses
- **Security group** — inbound from the load balancer only, never `0.0.0.0/0`

## Safety

- **Secrets never enter Terraform variables or state in plaintext.** Reference them from Secrets Manager or SSM by ARN.
- **`prevent_destroy` on stateful resources** — databases, buckets holding real data.
- **A destroy plan touching a stateful resource stops and asks.** Never apply it as part of a routine change.
