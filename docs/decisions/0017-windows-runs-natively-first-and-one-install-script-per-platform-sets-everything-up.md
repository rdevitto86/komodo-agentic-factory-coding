# 0017. Windows runs natively first, and one install script per platform sets everything up

**Status:** Accepted, 2026-09-25. Consolidated 2026-10-02 from 0017 and 0019.

**Context.** The owner wants the line to work out of the box with little setup. WSL2 needs a Windows feature turned on; native Windows needs only Git for Windows, which the line needs anyway. The host's sandbox runs in WSL2 but not natively. `komodo` isn't a command anyone has until it's installed.

**Decision.**

- **Ship a native `windows/amd64` binary and `install.ps1`.** Native Windows runs without the OS sandbox, and the other layers carry the load. Where WSL2 is present, the Linux install inside it gets the sandbox.
- **A job object kills process trees on native Windows,** and the guard's matcher covers PowerShell.
- **`install.sh` and `install.ps1` check prerequisites, build or download the binary, link it onto PATH,** install the global orchestrator layer, initialise the current repo and run doctor. Running one again updates the install.

**Alternatives.**

- **WSL2 only.** More setup than the owner wants.
- **Manual steps in the README.** Every machine drifts.

**Consequences.**

- **Native Windows is the one platform without an OS sandbox,** and doctor says so.
- **The first install also initialises the repo it runs in.**

**Spikes.** S6: does the line run natively on Windows 10 and 11 with Git for Windows, and inside WSL2 where present? Its result is recorded here once it runs.

**Open.** No job object or process watcher exists yet: `internal/proc/process_windows.go` kills only the parent. Build them, or narrow the native-Windows promise and REQ-43.
