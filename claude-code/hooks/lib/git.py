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
