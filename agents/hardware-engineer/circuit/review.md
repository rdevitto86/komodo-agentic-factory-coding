# Skill: /circuit-review

Review an electrical circuit design or schematic for correctness, safety, and manufacturability.

## Usage

```
/circuit-review <file-or-description> [--power|--signal|--mixed] [--safety-critical]
```

- `<file-or-description>` — Path to a schematic file, netlist, or a text description of the circuit.
- `--power` — Focus review on power delivery, regulation, and protection.
- `--signal` — Focus review on signal integrity, filtering, and analog/digital interfaces.
- `--mixed` — Full review covering both power and signal domains.
- `--safety-critical` — Apply heightened scrutiny for medical, industrial, or safety-rated applications.

---

## Review checklist

### Power supply and protection
- [ ] Decoupling capacitors present and correctly placed (bulk + ceramic, close to IC supply pins)
- [ ] Reverse polarity protection (diode, P-channel FET, or polyfuse)
- [ ] Overvoltage and overcurrent protection where applicable
- [ ] Inrush current limiting on capacitive loads
- [ ] Power sequencing correct for multi-rail ICs
- [ ] Ground plane continuity and split-plane considerations

### Component selection
- [ ] All components within rated voltage, current, and temperature range
- [ ] Tolerance stack-up acceptable for critical values (R dividers, RC timing, oscillators)
- [ ] Substitution-safe: flag any single-source components
- [ ] Passive values standard (E96/E24 series) for production sourcing

### Signal integrity
- [ ] Pull-up/pull-down resistors on open-drain/open-collector outputs
- [ ] Series termination on high-speed signals
- [ ] Differential pair routing recommendations noted
- [ ] Filtering on ADC inputs (anti-aliasing)
- [ ] ESD protection on exposed I/O (USB, connectors, test points)

### Design for manufacture (DFM)
- [ ] No components with pitch < 0.5mm without explicit justification
- [ ] Test points on key nets (power rails, UART, SWD/JTAG)
- [ ] Connector polarization (keyed or clearly marked)
- [ ] Silkscreen designators legible and not obscured by pads

### Safety (elevated for `--safety-critical`)
- [ ] Isolation barriers present where required (mains, high voltage, patient contact)
- [ ] Creepage and clearance distances meet IEC 60950 / IEC 61010 / relevant standard
- [ ] Fusing on mains input and high-current outputs
- [ ] Flammability ratings on PCB material and enclosure

---

## Output format

Provide findings in three sections:

1. **Critical** — Must fix before fabrication. Safety or functional failures.
2. **Major** — Should fix. Reliability, cost, or DFM issues.
3. **Minor** — Consider fixing. Best practice deviations, optimizations.

For each finding: state the net/component, the issue, and the recommended fix.

---

## After reviewing

Remind the designer to:
- Run ERC (Electrical Rules Check) in their EDA tool after applying fixes.
- Perform a BOM check for component availability and pricing before layout.
- For `--safety-critical` designs: engage a certified third-party for formal compliance testing.
