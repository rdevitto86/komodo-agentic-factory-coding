# Skill: /new-bom

Generate or review a Bill of Materials (BOM) for a hardware assembly.

## Usage

```
/new-bom <project> [--draft|--production] [--csv|--markdown]
```

- `<project>` — Project or assembly name, e.g. `sensor-node-v2`, `controller-board`.
- `--draft` — Generate a draft BOM template with placeholder rows for each component category.
- `--production` — Review an existing BOM for production readiness (sourcing, alternates, lead times).
- `--csv` — Output in CSV format suitable for import into a procurement tool.
- `--markdown` — Output as a Markdown table (default).

---

## Before generating anything

1. If reviewing an existing BOM, read it in full first.
2. Ask for the target production volume (prototype / NPI / volume) — this affects component selection and sourcing strategy.
3. Ask for any supply chain constraints (preferred distributors, approved vendor list, geographic restrictions).

---

## BOM columns

| Column | Description |
|--------|-------------|
| `Item` | Line item number |
| `Designator` | Reference designator(s) from schematic (e.g. C1, C2) |
| `Qty` | Quantity per assembly |
| `Description` | Component description (value, package, key specs) |
| `MPN` | Manufacturer Part Number |
| `Manufacturer` | Manufacturer name |
| `Alt MPN` | Approved alternate part number |
| `Distributor` | Preferred distributor (Digi-Key, Mouser, Arrow, etc.) |
| `Distributor PN` | Distributor part number |
| `Unit Cost` | Cost at target volume (leave blank for draft) |
| `Ext Cost` | Extended cost (Qty × Unit Cost) |
| `Notes` | Lead time, MOQ, or special handling notes |

---

## Production readiness review (--production)

Check each line for:
- [ ] MPN is complete and unambiguous (not just a generic description)
- [ ] At least one approved alternate MPN for every active component
- [ ] Distributor stock check — flag any components with <90 days stock at target volume
- [ ] Long lead-time components identified (>12 weeks) with recommended buffer stock
- [ ] Lifecycle status: flag any components marked "NRND" (Not Recommended for New Design) or "Obsolete"
- [ ] MOQ vs. build quantity alignment — flag cases where MOQ forces excess >20% of build quantity
- [ ] Consigned vs. purchased items clearly marked if applicable

---

## Draft BOM categories

For a `--draft`, generate placeholder sections for:
1. Power (regulators, capacitors, inductors)
2. Microcontrollers / processors
3. Memory (Flash, SRAM, EEPROM)
4. Communication ICs (wireless, wired interfaces)
5. Sensors
6. Passives (resistors, capacitors, inductors by value range)
7. Connectors and mechanical
8. Crystals and oscillators
9. Protection (TVS, fuses, ESD)
10. Indicators (LEDs, displays)
11. Miscellaneous (test points, mounting hardware)

---

## After generating

1. Print the BOM and total estimated cost (if unit costs provided).
2. Remind the team to:
   - Verify all MPNs against the live schematic before submitting to a CM.
   - Run a sourcing check on Octopart or similar before finalizing.
   - Store the approved BOM in version control alongside the schematic.
