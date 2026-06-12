# Embedded C/C++ Skill (robotics)

Shared embedded C/C++ baseline (domain, pre-flight checks, firmware idioms, flag list, output format) lives in `~/.claude/modes/cpp/coding.md` — read it first; this file holds only robotics-specific deltas.

---

## Robotics-specific deltas

- ROS 2 node scaffolding (C++ or Python) is its own skill: `~/.claude/agents/hardware-engineer/robotics/new-ros-node.md`. Use it when creating new nodes.
- Control-loop code (motor drivers, sensor fusion, kinematics) sits on top of the embedded baseline — apply the same ISR/timing/resource discipline, plus:
  - Treat control-loop rate as a hard real-time deadline; profile before merging changes that touch the hot path.
  - Sensor and actuator interfaces validate ranges/units at the boundary — a bad reading must not propagate into a control law unchecked.
- Where ROS message/service definitions cross the firmware boundary, document the serialization cost and any copies in the hot path.
