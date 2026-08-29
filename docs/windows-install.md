# Installing on Windows

This guide walks through installing this toolkit on a Windows machine, one step at a time. It assumes no prior familiarity with symlinks, PATH, environment variables, or Developer Mode — each is explained the first time it comes up.

If you're on macOS or Linux, use `bash setup.sh` instead — see the main [README](../README.md#setup).

## What you need first

- **Python.** The installer is a Python script, so Python has to already be on your machine. If you're not sure, open a terminal (PowerShell or Command Prompt both work) and try each of these one at a time:
  ```
  python3 --version
  python --version
  py -3 --version
  ```
  Whichever one prints a version number (3.x) instead of an error is the one you have. Different Windows Python installs name the program differently — `python3`, `python`, or the `py` launcher — so it's normal for only one of the three to work. If none of them work, install Python from [python.org](https://www.python.org/downloads/) and try again.
- **Git for Windows** (recommended, not required). This project's hooks are dispatched through a Unix-style shell called Git Bash, which comes bundled with [Git for Windows](https://gitforwindows.org/). Without it, some of the automation that keeps this toolkit's guardrails working may not run correctly. If you're already using Git on Windows, you likely have this already.

## Running the installer

Open a terminal in this repository's folder and run the command that matched a real version number above, for example:

```
python3 scripts/install.py
```

(substitute `python` or `py -3` if that's the one that worked for you)

This copies the toolkit's configuration into your Claude Code settings folder. What happens next depends on one thing: whether Windows will let the installer create a **symlink**.

### What a symlink is, and why it matters here

A symlink is a shortcut file that acts exactly like the real file or folder it points to — when the original changes, anything reading the symlink sees the change immediately, with nothing extra to do. This toolkit's normal way of installing is to symlink its `claude-code/` folder straight into your Claude Code settings folder, so any future update to this repository shows up automatically the next time you start a session.

Windows treats creating a symlink as a sensitive operation. By default, only an administrator account (or an account with a Windows feature called **Developer Mode** turned on) is allowed to create one. If you run the installer without either of those, it can't create the symlink — see the next section for what happens instead.

### Turning on Developer Mode (recommended)

Developer Mode is a Windows setting that allows ordinary user accounts to do a handful of things normally reserved for administrators, including creating symlinks. Turning it on once means every future install and re-install just works, with nothing to repeat.

To turn it on:

1. Open **Settings**.
2. Go to **Privacy & security**.
3. Click **For developers**.
4. Turn on **Developer Mode**.

Then run the installer command again (`python3 scripts/install.py`, or whichever interpreter name worked for you). This time it should be able to create the symlink.

### If you skip Developer Mode: copy-fallback mode

If Developer Mode isn't on and you're not running as an administrator, the installer doesn't fail — it falls back to **copying** the files instead of symlinking them, and prints a notice telling you it did so. This is called copy-fallback mode.

The difference matters for one reason: a copy is a snapshot, frozen at the moment it was made. If this repository's `claude-code/` folder is later updated (by pulling new changes, or by an agent editing a skill file), your installed copy will not pick up those changes on its own — you have to **re-sync**, meaning you re-run the install so it copies the newer files over the old ones.

The installer's copy-fallback notice prints the exact re-sync command to use — it's the same install command you ran the first time. Run it again any time you know `claude-code/` has changed and you want your installed copy to catch up.

If you'd rather not think about this at all, go back and turn on Developer Mode (previous section) so future installs use real symlinks instead.

## Verifying the install worked

1. Fully quit and restart Claude Code, so it picks up the newly installed settings.
2. Start a new session and make a small edit to a file that would normally trigger one of this toolkit's guardrails — for example, try adding a comment to a code file. If the install worked, the comment guard hook should step in and deny it, the same way it's documented to behave in this repository's [README](../README.md#the-hooks).

If nothing happens — no hook fires, no denial, no message — the install likely didn't take effect. Re-run the installer command and confirm it prints "installing agent config" with a target path pointing at your Claude Code settings folder, then restart Claude Code again.
