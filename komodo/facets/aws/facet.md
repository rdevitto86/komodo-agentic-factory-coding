# AWS

## Builder appendix
- Roles for workloads, users for humans; never a hardcoded access key.
- Start IAM policies and security groups from zero and add only what the workload proves it needs.
- Every new resource gets a lifecycle owner and a teardown plan before it is provisioned.

## Reviewer appendix
- Reject a policy widened from zero to `*` instead of the resource and action the workload needs.
- Reject `0.0.0.0/0` ingress on anything but a public-facing load balancer.
- Flag a resource with no lifecycle owner or teardown plan.
