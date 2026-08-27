# Runbook — <Title>

- **Owns:** <the SDD §7 signal or §10 recovery scenario this backs>
- **Severity:** <Sev1-4, or the on-call tier this pages>
- **Last verified:** <YYYY-MM-DD — the date someone last ran this for real or in a drill>

## Trigger
<The alert, symptom, or request that starts this runbook. Concrete: an alarm name, an error signature, a user-visible symptom — not "something is wrong.">

## Diagnosis
<Numbered steps to confirm this is actually the failure mode this runbook covers, not a lookalike. Each step names a command or dashboard, not just an instruction to "check X.">

## Resolution
<Numbered steps to fix it. Call out anything irreversible before the step that does it. State the expected outcome after each step so the operator knows whether to proceed or escalate.>

## Escalation
<Who to page and when — after which failed step, or which severity — and what context to hand them.>

## Related
<Links: the SDD §7/§10 section this backs, related §11 decisions, related runbooks.>
