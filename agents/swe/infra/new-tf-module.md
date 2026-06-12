# Skill: /new-tf-module

Scaffold a new Terraform module for a Komodo service or shared infrastructure component.

## Usage

```
/new-tf-module <name> <type> [--fargate|--lambda]
```

- `<name>` — Module name in kebab-case, e.g. `cart-api`, `shared-vpc`, `orders-queue`.
- `<type>` — Module type: `service` (ECS Fargate or Lambda service), `queue` (SQS), `storage` (S3 or DynamoDB), `network` (VPC/subnets), or `shared` (reusable module).
- `--fargate` or `--lambda` — Compute target for `service` type modules.

**Must be run from inside `infra/`.**

---

## Before generating anything

1. List `infra/modules/` to see existing module structure and naming patterns.
2. Read `infra/modules/<closest-module>/` to understand variable, output, and resource naming conventions.
3. Read `infra/environments/` to understand how modules are instantiated per environment.
4. For `service` type: read the service's `docker-compose.yaml` to get port, memory limits, and environment variable names.

---

## Files to generate

### `modules/<name>/main.tf`

```hcl
# Module: <name>
# Type: <type>
# TODO: implement resources

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# TODO: add resources
```

---

### `modules/<name>/variables.tf`

```hcl
variable "env" {
  description = "Deployment environment (local, staging, prod)"
  type        = string
}

variable "app_name" {
  description = "Application name"
  type        = string
}

# TODO: add module-specific variables
```

---

### `modules/<name>/outputs.tf`

```hcl
# TODO: export resource ARNs, IDs, and DNS names needed by other modules
```

---

### `modules/<name>/README.md`

```markdown
# <name>

TODO: describe what this module provisions.

## Inputs

| Name | Description | Required |
|------|-------------|----------|
| env | Deployment environment | yes |
| app_name | Application name | yes |

## Outputs

| Name | Description |
|------|-------------|
| TODO | TODO |
```

---

## Service module pattern (Fargate)

For `--fargate` service modules, include stubs for:
- `aws_ecs_task_definition` with container definition referencing the ECR image
- `aws_ecs_service` with desired count, load balancer, and service discovery
- `aws_cloudwatch_log_group` for container logs
- IAM task execution role with `AmazonECSTaskExecutionRolePolicy`
- Security group allowing inbound on the service port from the ALB only

---

## After generating

1. Print all files created.
2. Remind the developer to:
   - Run `terraform validate` from the module directory.
   - Instantiate the module in `infra/environments/<env>/main.tf` before applying.
   - Run `terraform plan` against a non-production environment first.
   - Tag all resources with `env`, `app_name`, and `managed_by = "terraform"`.
