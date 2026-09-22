# Azure

## Builder appendix
- Managed identities for workloads, users for humans; never a hardcoded client secret.
- Start role assignments and NSGs from zero and add only what the workload proves it needs.
- Every new resource gets a lifecycle owner and a teardown plan before it is provisioned.

## Reviewer appendix
- Reject an assignment widened to Owner or Contributor instead of the narrowest built-in role.
- Reject unrestricted inbound (`0.0.0.0/0`/`Any`) on anything but a public-facing load balancer or gateway.
- Flag a resource with no lifecycle owner or teardown plan.
