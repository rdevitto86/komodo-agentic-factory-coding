---
name: cpp
description: Embedded C/C++ — ISR discipline, static allocation, bounded resources, timing. Load before reading or writing any .c, .h, .cc, .cpp, .hpp, or CMakeLists.txt file.
user-invocable: false
---

# Embedded C/C++

Firmware constraints that do not apply to hosted software. **Where an embedded constraint conflicts with general guidance, the embedded constraint wins — name the conflict when it happens.**

## Domain

- Bare-metal and RTOS firmware (FreeRTOS, Zephyr)
- ARM Cortex-M, AVR, RP2040, ESP32
- Device drivers and peripheral control; hardware-software integration
- Memory and power management; cross-compilation toolchains
- Debugging via JTAG/SWD, logic analysers, oscilloscopes
- Safety-critical standards (IEC 61508, ISO 26262, DO-178C) where relevant

## Confirm before writing

Ask rather than assume — a wrong guess here costs a hardware revision.

- **Target**: chip family, RTOS or bare-metal, toolchain, debugger.
- **Timing**: deadlines, interrupt latency budgets, control-loop rates.
- **Resources**: flash size, RAM, stack budget per task, power envelope.
- **Read the existing drivers and HAL** before writing a new abstraction.

## How to write firmware

- **Keep ISRs short.** Defer work to a task or DPC. Never block in an ISR.
- **Bound everything.** No unbounded queues, no unbounded loops without a watchdog.
- **Static allocation by default.** Heap allocation needs a justification.
- **DMA for high-bandwidth peripherals** when latency or CPU budget matters — establish the buffer lifetime explicitly.
- **Host-side unit tests for pure logic**, colocated. HIL for timing and peripheral behaviour. On-target runs for end-to-end.
- **Merge/release gate structure follows the `sdlc` skill** where hosted-software tiers apply; HIL and on-target runs are the embedded substitute for the deployed-environment tiers.

## Flag aggressively

- Missing watchdog, or a watchdog kicked on only one code path
- Shared state between ISR and task without atomic access or a critical section
- Floating-point in ISR context on hardware without FPU lazy-stacking
- Blocking calls inside time-critical loops
- Stack growth approaching the configured per-task limit
