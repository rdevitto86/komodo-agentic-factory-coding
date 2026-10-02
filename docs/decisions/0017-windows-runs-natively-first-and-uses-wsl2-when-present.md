# 0017. Windows runs natively first, and uses WSL2 when present

**Status:** Accepted, 2026-09-25.

**Context.** The owner wants the line to work out of the box with little setup. WSL2 needs a Windows feature turned on; native Windows needs only Git for Windows, which the line needs anyway. The host's sandbox runs in WSL2 but not natively.

**Decision.** Ship a native `windows/amd64` binary and `install.ps1`. Native Windows runs without the OS sandbox, and the other layers carry the load. Where WSL2 is present, the Linux install inside it gets the sandbox. A job object kills process trees on native Windows.

**Alternatives.**

- **WSL2 only.** More setup than the owner wants.

**Consequences.**

- **Native Windows is the one platform without an OS sandbox,** and doctor says so.
- **The guard's matcher covers PowerShell.**

**Spikes.** S6: does the line run natively on Windows 10 and 11 with Git for Windows, and inside WSL2 where present?
