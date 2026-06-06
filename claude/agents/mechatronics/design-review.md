# Skill: /design-review

Review a mechanical design or assembly for functionality, manufacturability, and reliability.

## Usage

```
/design-review <file-or-description> [--structural|--thermal|--dfm|--all]
```

- `<file-or-description>` — Path to a CAD export, drawing, or text description of the design.
- `--structural` — Focus on load paths, stress concentrations, and fastener selection.
- `--thermal` — Focus on heat dissipation, thermal interfaces, and expansion/contraction.
- `--dfm` — Focus on design for manufacture and assembly (machining, injection molding, sheet metal).
- `--all` — Full review across all domains (default if no flag provided).

---

## Review checklist

### Structural integrity
- [ ] Load path is continuous and clearly defined
- [ ] No sharp internal corners in high-stress regions (specify fillet radii)
- [ ] Fastener selection appropriate for load type (shear vs. tension vs. vibration)
- [ ] Thread engagement depth meets minimum (typically 1.5× diameter for steel into aluminum)
- [ ] Clearance fits vs. interference fits correctly specified on drawings
- [ ] Thin walls or ribs likely to deflect under load identified

### Thermal management
- [ ] Heat-generating components have defined thermal path to ambient
- [ ] Thermal interface materials specified where applicable
- [ ] Differential thermal expansion between dissimilar materials accounted for
- [ ] Ventilation slots or openings sized for required airflow (if passive cooling)

### Design for manufacture (DFM)
- [ ] All features machinable with standard tooling (undercuts, deep pockets, tight tolerances flagged)
- [ ] Draft angles specified on injection-molded or cast parts (minimum 1° per face)
- [ ] Sheet metal bend radii ≥ material thickness; no features too close to bends
- [ ] Tolerances achievable with specified process (GD&T / ISO 2768 appropriateness)
- [ ] Assembly sequence is logical with no inaccessible fasteners

### Drawing completeness
- [ ] All critical dimensions toleranced
- [ ] Surface finish requirements specified
- [ ] Material and treatment callouts complete (alloy, temper, coating, hardness)
- [ ] Title block: part number, revision, units, projection angle, scale

---

## Output format

Provide findings in three sections:

1. **Critical** — Functional or safety failures; must be resolved before fabrication.
2. **Major** — Manufacturability or reliability issues; strongly recommended to fix.
3. **Minor** — DFM optimizations, drawing completeness, or best practice suggestions.

For each finding: reference the feature or view, describe the issue, and suggest the fix.

---

## After reviewing

Remind the designer to:
- Update the drawing revision and document changes in the revision block.
- Re-run FEA on any geometry changes if structural findings were addressed.
- Obtain a first-article inspection (FAI) report from the CM for critical assemblies.
