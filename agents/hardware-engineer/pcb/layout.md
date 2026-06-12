# PCB Layout Review

General checklist for reviewing a PCB layout before fabrication. Use alongside `circuit/review.md` for schematic-level findings.

---

## Power and protection

- Decoupling capacitors: bulk + ceramic at every IC, values per datasheet recommendations
- ESD protection on all external-facing pins
- Overvoltage and reverse-polarity protection where applicable
- Fusing or PTC on battery and USB power rails

## Signal integrity

- Trace widths sized for current (IPC-2221)
- Differential pairs length-matched and routed together
- High-speed signals (USB, SPI, CAN) with return path vias
- No 90° bends on high-frequency traces

## Thermal

- Worst-case dissipation calculated for max ambient
- Thermal relief on pads connected to ground planes
- Heatsink or copper pours flagged where T_J is a concern

## Ground plane

- Single reference unless split is required (mixed-signal analog/digital)
- Guard rings on sensitive analog inputs adjacent to noisy digital signals

## DFM

- Trace/space within manufacturer capability
- Component courtyard clearances met
- Silkscreen legibility and fiducial marks present
- Panelization considerations noted for production

---

## Robotics-specific addendum

For boards in a robotics power/motion stack, also check:

- Power regulation sized for motor inrush and stall current, not just steady-state draw
- Motor driver layout: gate drive traces short and direct, current-sense shunt placement, snubber/flyback protection on inductive loads
- EMI mitigation: motor and driver switching noise isolated from sensor and logic domains (ground splits, ferrite beads, shielding)
- Connector selection rated for the vibration and current environment (locking connectors for moving assemblies)
- Signal integrity on encoder/sensor lines routed away from motor phase traces

---

## Output format

Structure findings as:
- **Critical** — safety violation or likely field failure
- **Major** — functional issue, will not work correctly
- **Minor** — best-practice deviation, DFM risk, or improvement

Cite the specific component, reference designator, or net for each finding. Flag safety-critical issues separately and note which certification standard applies.
