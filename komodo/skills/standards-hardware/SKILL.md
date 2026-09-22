---
name: standards-hardware
description: Schematic and PCB practice: nets, footprints, power, and design-rule discipline.
globs: ["**/*.brd", "**/*.kicad_pcb", "**/*.kicad_sch", "**/*.sch"]
---

# Hardware

Physical design and integration. Firmware code rules live in the cpp standard — this covers everything upstream of it.

## Confirm before designing

A wrong assumption here costs a board spin, not a rebuild. Ask:

- **Supply**: input voltage range, current budget, battery or mains, expected duty cycle.
- **Environment**: temperature range, humidity, vibration, ingress rating.
- **Compliance**: which certifications apply — EMC, safety, RF.
- **Volume**: prototype, small run, or production. It changes every part choice.

## Circuit review

- **Decoupling on every supply pin**, sized and placed close.
- **Protection at every external interface** — TVS, series resistance, reverse polarity, ESD.
- **Define every pin's power-on state.** A floating input at reset is a real failure mode.
- **Verify current paths under fault**, not just nominal.
- **Check the thermal budget** for anything dissipating meaningful power.

## PCB layout

- **Ground plane integrity first.** A split or slotted return path under a fast signal causes problems no schematic review finds.
- **Keep return paths short and adjacent** to their signal.
- **Separate analogue, digital, and switching sections**, with a deliberate single connection point.
- **Route differential pairs together**, length-matched, impedance-controlled where the standard requires it.
- **Keep the crystal loop tight** and away from switching nodes.

## BOM

- **Every line carries a manufacturer part number**, not just a value. A "10k resistor" is not a BOM line.
- **Check lifecycle status and lead time** on every active component. Flag anything NRND or single-sourced.
- **Name a second source** for anything that could gate a build.
- **State tolerance, voltage rating, and package** explicitly — these are where substitutions silently fail.

## Robotics integration

- **Establish the coordinate frames and their transforms** before anything else.
- **State control loop rates and the latency budget** end to end, including sensing and actuation.
- **Define the safe state and how it is reached** on every failure path — loss of power, loss of comms, sensor fault.
- **Emergency stop is hardware, not software.** Never propose a software-only stop path.
