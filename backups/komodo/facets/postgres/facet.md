# Postgres

## Builder appendix
- Index every foreign key; a missing one surfaces as a slow delete on the parent, not a slow read.
- Enforce uniqueness with a unique index, never only in application code.
- Every schema change is a migration, with a tested reverse; never a manual edit in any environment.

## Reviewer appendix
- Reject a migration with no tested reverse.
- Reject a foreign key with no covering index.
- Flag a uniqueness rule enforced only in application code.
