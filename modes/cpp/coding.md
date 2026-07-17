# Embedded C/C++ Skill

Embedded and bare-metal firmware development — the constraints, idioms, and failure modes that don't apply to hosted software. General coding doctrine (hard rules, error strings, DI, testability) still applies from `~/.claude/standards/principles.md`; where an embedded constraint conflicts with general guidance, the embedded constraint wins — name the conflict when it happens.

---

## 1. Domain

- Bare-metal and RTOS firmware (FreeRTOS, Zephyr)
- Microcontrollers: ARM Cortex-M, AVR, RP2040, ESP32
- C/C++ and Rust for embedded; Python for host-side tooling
- Device drivers and peripheral control; hardware-software integration
- Memory and power management; cross-compilation toolchains
- Debugging via JTAG/SWD, logic analyzers, oscilloscopes
- Safety-critical standards (IEC 61508, ISO 26262, DO-178C) — apply when relevant

---

## 2. Before starting

- Confirm target: chip family, RTOS or bare-metal, toolchain, debugger.
- Confirm timing constraints: deadlines, interrupt latency budgets, control-loop rates.
- Confirm resource constraints: flash size, RAM, stack budget per task, power envelope.
- Read the existing drivers and HAL code in the project before writing new abstractions.

---

## 3. How you write firmware

- Follow TDD where the platform allows: host-side unit tests for pure logic, colocated with the source. HIL for timing and peripheral behaviour — if the repo also has hosted components already using a top-level `test/` tree, HIL tests can live under `test/hil/` for consistency; a firmware-only repo with no such tree has no reason to adopt one. E2e on target hardware.
- Keep ISRs short — defer work to a task or DPC; never block in an ISR.
- Bounded everything — no unbounded queues, no unbounded loops without a watchdog.
- DMA for high-bandwidth peripherals when latency or CPU budget matters; document the buffer lifetime.
- Static allocation by default; heap allocation only with justification.

---

## 4. Flag aggressively

- Missing watchdog, or a watchdog kick on only a single code path
- Shared state between ISR and task without atomic access or a critical section
- Floating-point in ISR context on hardware without FPU lazy-stacking
- Blocking calls inside time-critical loops
- Stack growth that approaches the configured per-task limit

---

## 5. Output

When done, summarize:
1. What changed and why
2. Timing/resource impact (cycles, flash, RAM) where measurable
3. Test coverage: what is unit-tested, what is HIL-tested, what is unverified and why
4. Follow-ups with enough context to ticket
