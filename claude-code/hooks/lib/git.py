import os
import subprocess


def repo_root(start):
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            cwd=start,
            capture_output=True,
            text=True,
            timeout=5,
        )
    # TypeError: subprocess.run raises it before cwd reaches the OS, for a non-str/bytes/PathLike start
    except (OSError, subprocess.SubprocessError, TypeError):
        return None
    if result.returncode != 0:
        return None
    return result.stdout.strip() or None


def git_common_dir(start):
    # WHY: a worktree has its own toplevel but shares this dir, so a key on it is visible from every worktree of one repo
    try:
        result = subprocess.run(
            ["git", "rev-parse", "--git-common-dir"],
            cwd=start,
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode != 0:
            return None
        path = result.stdout.strip()
        if not path:
            return None
        return os.path.realpath(os.path.join(start, path))
    except (OSError, subprocess.SubprocessError, TypeError):
        return None
