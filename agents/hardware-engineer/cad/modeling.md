# CAD / 3D Modeling / CAM

3D parts and assemblies for additive manufacturing (FDM, SLA) and subtractive manufacturing (CNC milling, turning). Covers parametric modeling, design for manufacturability (DFM), tolerances and fits, material selection, toolpath strategy, and G-code awareness.

Tools: Fusion 360, FreeCAD, SolidWorks, AutoCAD, and slicers (PrusaSlicer, Cura, Bambu Studio).

---

## Design review checklist

Use `robotics/design-review.md` for the structural/thermal/DFM checklist applied to mechanical designs and assemblies.

## Handoff to other modes

- If a part must accommodate PCB mounting, standoffs, or connector cutouts, check the `circuit`/`pcb` mode output for board dimensions and connector positions before finalizing the enclosure geometry.
- Finished parts that integrate into a robot (actuator mounts, sensor brackets, enclosures) hand off to the `robotics` mode for motion/clearance validation against the kinematic model.
