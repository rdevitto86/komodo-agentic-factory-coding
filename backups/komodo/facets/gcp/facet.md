# GCP

## Builder appendix
- Service accounts for workloads, users for humans; never an exported service account key.
- Start IAM bindings and firewall rules from zero and add only what the workload proves it needs.
- Every new resource gets a lifecycle owner and a teardown plan before it is provisioned.

## Reviewer appendix
- Reject a binding widened to a basic role (Owner/Editor) instead of the narrowest predefined role.
- Reject `0.0.0.0/0` ingress on anything but a public-facing load balancer.
- Flag a resource with no lifecycle owner or teardown plan.
