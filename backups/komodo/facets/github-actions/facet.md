# GitHub Actions

## Builder appendix
- A secret lives in GitHub's encrypted secrets store, never in a workflow file or its logs.
- Cancel superseded runs on the same ref; never on the default branch.
- One command is a workflow's entry point on every leg; a step that branches on `runs-on` is testing the branch, not the code.

## Reviewer appendix
- Reject a secret or token written into a workflow YAML file instead of `secrets.*`.
- Reject a workflow with no concurrency group cancelling superseded runs on a feature ref.
- Flag a required check a workflow does not actually gate.
