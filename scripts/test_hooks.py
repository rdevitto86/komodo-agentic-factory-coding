#!/usr/bin/env python3
from __future__ import annotations

import glob
import hashlib
import json
import os
import platform
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HOOKS = os.path.join(REPO_ROOT, "claude-code", "hooks")
GIT_GUARD = os.path.join(HOOKS, "git_guard.py")
AUTO_FORMAT = os.path.join(HOOKS, "auto_format.py")
COMMENTS = os.path.join(HOOKS, "comments.py")
VERIFY_GATE = os.path.join(HOOKS, "verify_gate.py")

PYTHON = sys.executable or shutil.which("python3")
SESSION = "hooktest%d" % os.getpid()
DEFAULT_PARALLEL = 8
DEEP_NESTING_DEPTH = 3000
SKIP_TICKET = "TSK-01.1.14"
MAKE_SKIP_REASON = "make unavailable, or Windows runner"
GO_FIXTURE = "package main\n\nfunc Run() {\n\tx := 1\n\treturn x\n}\n"
DISCRIMINANT_FIXTURE = (
    "package main\n\nfunc (c *secretCache) getParsed(key string) "
    "(map[string]string, bool) {\n\treturn nil, false\n}\n"
)

WORKDIR = [""]
FIXTURES = {}
RESULTS = {}
LOCK = threading.Lock()
STATE = {"index": 0}

INJECT_SCRIPT = '''
import os, sys, io
sys.path.insert(0, os.environ["HOOKS"])
import context_injector as ci
ci.repo_root = lambda start: os.environ["INJECT_ROOT"] or None
buf = io.StringIO()
sys.stdout = buf
try:
    ci.main()
except SystemExit:
    pass
except BaseException:
    sys.stdout = sys.__stdout__
    print("CRASHED")
    raise SystemExit(0)
sys.stdout = sys.__stdout__
print(buf.getvalue(), end="")
'''

FORMATTER_SCRIPT = '''
import io
import os
import sys

sys.path.insert(0, os.environ["HOOKS"])
import comments as wcv

calls = []
wcv.run_formatter = lambda path: calls.append(path)

fixture = os.environ["FIXTURE_VALIDATOR"]
proposals = [{
    "file": "svc.go",
    "line": 4,
    "template_type": "WHY",
    "text": "// confirms run_formatter fires after a splice",
}]

buf = io.StringIO()
sys.stdout = buf
try:
    wcv.process_proposals(proposals, fixture)
except BaseException:
    sys.stdout = sys.__stdout__
    print("CRASHED")
    raise SystemExit(0)
sys.stdout = sys.__stdout__

expected = os.path.join(fixture, "svc.go")
if calls != [expected]:
    print("run_formatter calls=%r want=[%r]" % (calls, expected))
else:
    print("OK")
'''


def emit(text: str) -> None:
    sys.stdout.write(text)
    sys.stdout.flush()


def is_windows_shell() -> bool:
    if os.name == "nt":
        return True
    name = platform.system().upper()
    return name.startswith(("MINGW", "MSYS", "CYGWIN", "WINDOWS"))


IS_WINDOWS = is_windows_shell()


def skip_case(label: str, reason: str) -> None:
    emit("  SKIP  %s\n        %s (%s)\n" % (label, reason, SKIP_TICKET))


def allocate() -> int:
    STATE["index"] += 1
    return STATE["index"]


def report(index: int, label: str, problem: str, extra: str = "") -> None:
    if not problem:
        text = "  PASS  %s\n" % label
    else:
        text = "  FAIL  %s\n        %s\n" % (label, problem)
        if extra:
            text += "".join("        | %s\n" % line for line in extra.split("\n"))
    with LOCK:
        RESULTS[index] = text


def write_text(path: str, body: str) -> None:
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(body)


def read_text(path: str) -> str:
    try:
        with open(path, encoding="utf-8", errors="replace") as handle:
            return handle.read()
    except OSError:
        return ""


def write_script(path: str, body: str) -> None:
    write_text(path, body)
    os.chmod(path, 0o755)


def env_with(**overrides) -> dict:
    env = dict(os.environ)
    for key, value in overrides.items():
        env[key] = value
    return env


def run_process(args: list, payload: str = None, env: dict = None, cwd: str = None,
                quiet: bool = True):
    return subprocess.run(
        args,
        input=payload,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL if quiet else None,
        env=env,
        cwd=cwd,
        universal_newlines=True,
    )


def run_merged(args: list, payload: str = None, env: dict = None, cwd: str = None):
    return subprocess.run(
        args,
        input=payload,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=env,
        cwd=cwd,
        universal_newlines=True,
    )


def run_hook(hook: str, payload: str, env: dict = None, cwd: str = None,
             quiet: bool = True) -> str:
    return run_process([PYTHON, hook], payload, env, cwd, quiet).stdout


def git_quiet(args: list, cwd: str = None) -> None:
    run_process(["git"] + args, cwd=cwd)


def make_repo(path: str) -> None:
    if not os.path.isdir(path):
        os.makedirs(path)
    git_quiet(["init", "-q", "-b", "main", path])
    git_quiet(["config", "user.email", "t@t.com"], cwd=path)
    git_quiet(["config", "user.name", "t"], cwd=path)


def seeded_repo(path: str) -> None:
    make_repo(path)
    git_quiet(["commit", "-q", "--allow-empty", "-m", "init"], cwd=path)


def decision_reason(out: str):
    if not out:
        return "allow", ""
    try:
        block = json.loads(out)["hookSpecificOutput"]
        return block["permissionDecision"], block.get("permissionDecisionReason", "")
    except (ValueError, KeyError, TypeError):
        return "malformed", ""


def mismatch(got: str, want: str, reason: str, must_contain: str,
             must_not_contain: str = "") -> str:
    if got != want:
        return "decision=%s want=%s" % (got, want)
    if must_contain and must_contain not in reason:
        return "reason missing: %s" % must_contain
    if must_not_contain and must_not_contain in reason:
        return "reason leaked: %s" % must_not_contain
    return ""


def inject_event(payload: str) -> str:
    brace = payload.find("{")
    if brace < 0:
        return payload
    return '{"hook_event_name":"PreToolUse",' + payload[brace + 1:]


def bash_payload(command: str, cwd: str = None, agent: str = None) -> str:
    body = {"hook_event_name": "PreToolUse", "tool_name": "Bash"}
    if cwd is not None:
        body["cwd"] = cwd
    if agent is not None:
        body["agent_type"] = agent
    body["tool_input"] = {"command": command}
    return json.dumps(body)


POOL = {"executor": None}


def submit(fn, *args) -> None:
    POOL["executor"].submit(guarded(fn), *args)


def guarded(fn):
    def wrapper(*args):
        try:
            fn(*args)
        except BaseException as err:
            emit("  harness error: %r\n" % (err,))
    return wrapper


def decision_case(index: int, label: str, want: str, payload: str, must_contain: str,
                  must_not_contain: str = "", hook: str = None, cwd: str = None,
                  env: dict = None) -> None:
    out = run_hook(hook or GIT_GUARD, payload, env=env, cwd=cwd)
    got, reason = decision_reason(out)
    problem = mismatch(got, want, reason, must_contain, must_not_contain)
    report(index, label, problem, reason)


def bash_case(label: str, want: str, command: str, must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command, cwd=FIXTURES["main"])
    submit(decision_case, index, label, want, payload, must_contain)


def bash_case_cwd(directory: str, label: str, want: str, command: str,
                  must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command, cwd=directory)
    submit(decision_case, index, label, want, payload, must_contain)


def bash_case_agent(label: str, want: str, agent: str, command: str,
                    must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command, agent=agent)
    submit(decision_case, index, label, want, payload, must_contain)


def bash_case_agent_at(directory: str, label: str, want: str, agent: str, command: str,
                       must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command, agent=agent)
    submit(decision_case, index, label, want, payload, must_contain, "", None, directory)


def bash_case_at(directory: str, label: str, want: str, command: str,
                 must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command)
    submit(decision_case, index, label, want, payload, must_contain, "", None, directory)


def off_case(label: str, want: str, command: str, must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command)
    submit(decision_case, index, label, want, payload, must_contain, "", None, None,
           env_with(PUBLISH_ENABLED="0"))


def crash_case(label: str, want: str, command: str, must_contain: str = "") -> None:
    index = allocate()
    payload = bash_payload(command)
    submit(decision_case, index, label, want, payload, must_contain, "", None, None,
           env_with(GIT_GUARD_TEST_CRASH="1"))


def expect(label: str, want: str, payload: str, must_contain: str = "",
           must_not_contain: str = "") -> None:
    index = allocate()
    submit(decision_case, index, label, want, inject_event(payload), must_contain,
           must_not_contain, AUTO_FORMAT)


def smoke_case(label: str, want: str, command: str) -> None:
    payload = bash_payload(command, cwd=FIXTURES["main"])
    out = run_hook(GIT_GUARD, payload, quiet=False)
    got, reason = decision_reason(out)
    if got != want:
        sys.stderr.write(
            "  SMOKE FAIL  %s\n        decision=%s want=%s\n        %s\n"
            % (label, got, want, reason)
        )
        raise SystemExit(1)
    emit("  SMOKE PASS  %s\n" % label)


def check_body(index: int, label: str, name: str, body: str, must_contain: str,
               must_not_contain: str) -> None:
    target = os.path.join(FIXTURES["check"], "%d-%s" % (index, name))
    write_text(target, body.strip("\n") + "\n")
    out = run_merged(
        [PYTHON, COMMENTS, "--repo-root", FIXTURES["check"], "check", "--all", target]
    ).stdout
    problem = ""
    if must_contain and must_contain not in out:
        problem = "expected finding missing: %s" % must_contain
    if not problem and must_not_contain and must_not_contain in out:
        problem = "unexpected finding present: %s" % must_not_contain
    report(index, label, problem, out)


def check_case(label: str, name: str, must_contain: str, must_not_contain: str,
               body: str) -> None:
    index = allocate()
    submit(check_body, index, label, name, body, must_contain, must_not_contain)


def apply_proposals(proposals: str, root: str) -> str:
    return run_process(
        [PYTHON, COMMENTS, "--repo-root", root, "apply"], payload=proposals
    ).stdout


def splice_problem(out: str, root: str, must_spliced: str, must_dropped: str,
                   must_in_file: str) -> str:
    if must_spliced and not ordered_in(out, '"spliced"', must_spliced):
        return "spliced result missing: %s" % must_spliced
    if must_dropped and not ordered_in(out, '"dropped"', must_dropped):
        return "dropped reason missing: %s" % must_dropped
    if must_in_file and must_in_file not in read_text(os.path.join(root, "svc.go")):
        return "fixture file does not contain: %s" % must_in_file
    return ""


def ordered_in(haystack: str, *needles) -> bool:
    cursor = 0
    for needle in needles:
        found = haystack.find(needle, cursor)
        if found < 0:
            return False
        cursor = found + len(needle)
    return True


def validator_body(index: int, label: str, proposals: str, must_spliced: str,
                   must_dropped: str, must_in_file: str) -> None:
    out = apply_proposals(proposals, FIXTURES["validator"])
    problem = splice_problem(out, FIXTURES["validator"], must_spliced, must_dropped,
                             must_in_file)
    report(index, label, problem, out)


def validator_case(label: str, proposals: str, must_spliced: str = "",
                   must_dropped: str = "", must_in_file: str = "") -> None:
    index = allocate()
    submit(validator_body, index, label, proposals, must_spliced, must_dropped,
           must_in_file)


def own_fixture_body(index: int, label: str, proposals: str, must_spliced: str,
                     must_dropped: str, must_in_file: str) -> None:
    fixture = os.path.join(WORKDIR[0], "fixture-validator-%d" % index)
    os.makedirs(fixture)
    write_text(os.path.join(fixture, "svc.go"), GO_FIXTURE)
    out = apply_proposals(proposals, fixture)
    problem = splice_problem(out, fixture, must_spliced, must_dropped, must_in_file)
    report(index, label, problem, out)


def validator_own_fixture_case(label: str, proposals: str, must_spliced: str = "",
                               must_dropped: str = "", must_in_file: str = "") -> None:
    index = allocate()
    submit(own_fixture_body, index, label, proposals, must_spliced, must_dropped,
           must_in_file)


def custom_case(label: str, fn) -> None:
    index = allocate()
    submit(fn, index, label)


def v9b_body(index: int, label: str) -> None:
    fixture = FIXTURES["v9b"]
    out = apply_proposals(
        '[{"file":"svc.go","line":3,"template_type":"DOC",'
        '"text":"// helper does something small."}]',
        fixture,
    )
    problem = ""
    if not ordered_in(out, '"dropped"', "exported (capitalized) declaration"):
        problem = "dropped reason missing: exported (capitalized) declaration"
    report(index, label, problem, out)


def v11_body(index: int, label: str) -> None:
    fixture = FIXTURES["v11"]
    out = apply_proposals(
        '[{"file":"svc.go","line":4,"template_type":"FIELD",'
        '"text":"// zero disables the deadline entirely"},'
        '{"file":"svc.go","line":5,"template_type":"FIELD",'
        '"text":"// pulled from env at boot"}]',
        fixture,
    )
    first = ordered_in(out, '"spliced"', "zero disables the deadline entirely",
                       "pulled from env at boot")
    second = ordered_in(out, "pulled from env at boot",
                        "zero disables the deadline entirely")
    problem = ""
    if not first and not second:
        problem = ("both adjacent FIELD proposals should splice, neither treated as a "
                   "stack: %s" % out)
    elif "// zero disables the deadline entirely" not in read_text(
            os.path.join(fixture, "svc.go")):
        problem = "fixture file missing the spliced Timeout field comment"
    report(index, label, problem, out)


def v12_body(index: int, label: str) -> None:
    out = apply_proposals(
        '[{"file":"svc_test.go","line":3,"template_type":"BANNER",'
        '"text":"// --- Setup ---"}]',
        FIXTURES["v12"],
    )
    problem = ""
    if not ordered_in(out, '"spliced"', "--- Setup ---"):
        problem = "a Setup banner in a _test.go file should splice: %s" % out
    report(index, label, problem, out)


def v3_body(index: int, label: str) -> None:
    fixture = os.path.join(WORKDIR[0], "fixture-validator-fmt")
    if not os.path.isdir(fixture):
        os.makedirs(fixture)
    write_text(os.path.join(fixture, "svc.go"), GO_FIXTURE)
    out = run_merged(
        [PYTHON, "-c", FORMATTER_SCRIPT],
        env=env_with(HOOKS=HOOKS, FIXTURE_VALIDATOR=fixture),
    ).stdout.strip("\n")
    problem = ""
    if out != "OK":
        problem = "run_formatter was not invoked with the spliced file's path"
    report(index, label, problem, out)


def e2e_body(first: int, second: int, third: int) -> None:
    fixture = os.path.join(WORKDIR[0], "fixture-e2e")
    if not os.path.isdir(fixture):
        os.makedirs(fixture)
    target = os.path.join(fixture, "svc.go")
    write_text(
        target,
        "package main\n\nfunc getParsed(k string) (map[string]string, bool) {\n"
        "\treturn nil, false\n}\n",
    )

    out1 = run_merged(
        [PYTHON, COMMENTS, "--repo-root", fixture, "check", "--all", target]
    ).stdout
    problem1 = ""
    if "RET_BOOL_DISCRIMINANT" not in out1:
        problem1 = "check did not report the discriminant site"
    report(first, "E1  check reports the discriminant site before any comment exists",
           problem1, out1)

    proposals = ('[{"file":"svc.go","line":3,"template_type":"WHY","text":"// false '
                 'means the key was absent or its value failed to parse"}]')
    out2 = apply_proposals(proposals, fixture)
    problem2 = ""
    if not ordered_in(out2, '"spliced"', "false means the key was absent"):
        problem2 = "spliced result missing expected text"
    elif "false means the key was absent" not in read_text(target):
        problem2 = "fixture file does not contain the spliced WHY comment"
    report(second, "E2  apply splices the comment the check demanded", problem2, out2)

    out3 = run_merged(
        [PYTHON, COMMENTS, "--repo-root", fixture, "check", "--all", target]
    ).stdout
    problem3 = ""
    if "MISSING" in out3:
        problem3 = "check still reports a MISSING site after apply"
    report(third, "E3  the same check comes back clean once the comment landed",
           problem3, out3)


def untracked_body(index: int, label: str) -> None:
    fixture = os.path.join(WORKDIR[0], "fixture-untracked")
    seeded_repo(fixture)
    target = os.path.join(fixture, "svc.go")
    write_text(
        target,
        'package main\n\nfunc Split(s string) (string, string, error) {\n'
        '\treturn "", "", nil\n}\n',
    )
    out = run_merged(
        [PYTHON, COMMENTS, "--repo-root", fixture, "check", target]
    ).stdout
    problem = ""
    if "RET_ARITY_3" not in out:
        problem = ("a new untracked file with a RET_ARITY_3 site was not reported by a "
                   "default check: %s" % out)
    report(index, label, problem, out)


def comments_hook_body(index: int, label: str, name: str, source: str,
                       expect_findings: bool) -> None:
    fixture = os.path.join(WORKDIR[0], name)
    seeded_repo(fixture)
    write_text(os.path.join(fixture, "svc.go"), source)
    payload = json.dumps(
        {"tool_name": "Edit", "tool_input": {"file_path": os.path.join(fixture, "svc.go")}}
    )
    result = run_merged([PYTHON, COMMENTS, "--repo-root", fixture, "hook"], payload)
    out = result.stdout
    problem = ""
    if result.returncode != 0:
        problem = "exited %d instead of 0" % result.returncode
    elif expect_findings:
        if "additionalContext" not in out or "RET_BOOL_DISCRIMINANT" not in out:
            problem = "stdout missing additionalContext with the MISSING finding"
    elif out:
        problem = "a clean file produced output: %s" % out
    report(index, label, problem, out)


def hook_malformed_body(index: int, label: str) -> None:
    result = run_merged([PYTHON, COMMENTS, "hook"], "not json{{{")
    problem = ""
    if result.returncode != 0:
        problem = "exited %d instead of 0" % result.returncode
    elif result.stdout:
        problem = "a malformed payload produced output: %s" % result.stdout
    report(index, label, problem, result.stdout)


def hook_symlink_body(index: int, label: str) -> None:
    root_fixture = os.path.join(WORKDIR[0], "fixture-hook-root")
    foreign_fixture = os.path.join(WORKDIR[0], "fixture-hook-foreign")
    seeded_repo(root_fixture)
    seeded_repo(foreign_fixture)
    foreign_file = os.path.join(foreign_fixture, "svc.go")
    write_text(foreign_file, DISCRIMINANT_FIXTURE)
    os.symlink(foreign_file, os.path.join(root_fixture, "link.go"))
    payload = json.dumps(
        {"tool_name": "Edit",
         "tool_input": {"file_path": os.path.join(root_fixture, "link.go")}}
    )
    result = run_merged([PYTHON, COMMENTS, "--repo-root", root_fixture, "hook"], payload)
    problem = ""
    if result.returncode != 0:
        problem = "exited %d instead of 0" % result.returncode
    elif result.stdout:
        problem = ("a symlink resolving outside repo root produced output: %s"
                   % result.stdout)
    report(index, label, problem, result.stdout)


def auto_format_body(index: int, label: str, want_changed: str, path: str, tool: str,
                     path_override: str) -> None:
    before = read_text(path).rstrip("\n")
    payload = '{"tool_name":"%s","tool_input":{"file_path":"%s"}}' % (tool, path)
    env = env_with(PATH=path_override) if path_override else None
    result = subprocess.run(
        [PYTHON, AUTO_FORMAT],
        input=payload,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env=env,
        universal_newlines=True,
    )
    after = read_text(path).rstrip("\n")
    problem = ""
    if result.returncode != 0:
        problem = "hook exited %d instead of 0" % result.returncode
    elif want_changed == "yes" and before == after:
        problem = "file was not reformatted"
    elif want_changed == "no" and before != after:
        problem = "file was reformatted when it should have been left alone"
    report(index, label, problem)


def auto_format_case(label: str, want_changed: str, path: str, tool: str,
                     path_override: str = "") -> None:
    index = allocate()
    submit(auto_format_body, index, label, want_changed, path, tool, path_override)


def inject_body(index: int, label: str, root: str, must_contain: str,
                must_not_contain: str) -> None:
    out = run_hook_script(INJECT_SCRIPT, env_with(HOOKS=HOOKS, INJECT_ROOT=root))
    problem = ""
    if "CRASHED" in out:
        problem = "hook crashed instead of failing open"
    if not problem and must_contain and must_contain not in out:
        problem = "output missing: %s" % must_contain
    if not problem and must_not_contain and must_not_contain in out:
        problem = "output leaked: %s" % must_not_contain
    report(index, label, problem, out)


def run_hook_script(script: str, env: dict) -> str:
    return run_process([PYTHON, "-c", script], payload="", env=env).stdout


def inject_case(label: str, root: str, must_contain: str = "",
                must_not_contain: str = "") -> None:
    index = allocate()
    submit(inject_body, index, label, root, must_contain, must_not_contain)


def check_comments_check() -> None:
    emit("\ncomments check\n\n")

    check_case("K1  a bool discriminant return is a MISSING site", "svc.go",
               "RET_BOOL_DISCRIMINANT", "", """
package x

func (c *secretCache) getParsed(key string) (map[string]string, bool) {
\treturn nil, false
}
""")

    check_case("K2  an is-prefixed name is exempt", "svc.go", "0 finding", "RET_BOOL", """
package x

func isValid(key string) (string, bool) {
\treturn "", true
}
""")

    check_case("K3  a plain (T, error) return is not a site", "svc.go", "0 finding",
               "MISSING", """
package x

func Load(p string) (string, error) {
\treturn "", nil
}
""")

    check_case("K4  three return values is a MISSING site", "svc.go", "RET_ARITY_3", "", """
package x

func Split(s string) (string, string, error) {
\treturn "", "", nil
}
""")

    check_case("K5  an existing comment satisfies the site", "svc.go", "0 finding",
               "MISSING", """
package x

// false means the key was absent or unparseable
func lookup(k string) (map[string]string, bool) {
\treturn nil, false
}
""")

    check_case("K6  a func-typed parameter does not break the parser", "svc.go",
               "RET_BOOL_DISCRIMINANT", "", """
package x

func handler(cb func(int) error, x string) (map[string]string, bool) {
\treturn nil, false
}
""")

    check_case("K7  a named return tuple is parsed by type, not label", "svc.go",
               "RET_BOOL_DISCRIMINANT", "", """
package x

func fetch(k string) (out map[string]string, ok bool) {
\treturn nil, false
}
""")

    check_case("K8  a non-Go file yields no MISSING sites", "svc.py", "0 finding",
               "MISSING", """
def get_parsed(key):
    return None, False
""")

    check_case("K9  a name echo is INVALID", "svc.go", "NAME_ECHO", "", """
package x

// InitStore inits a store
func InitStore() {}
""")

    check_case("K10 a DOC-shaped comment on an exported func is not an echo", "svc.go",
               "0 finding", "NAME_ECHO", """
package x

// InitStore prepares the on-disk store and seeds it.
func InitStore() {}
""")

    check_case("K11 an over-cap comment is INVALID", "svc.go", "OVER_CAP", "", """
package x

// this single comment body runs well past the hundred and twenty character cap that the narrative rule sets for one line of prose
func Run() {}
""")

    check_case("K12 a machine directive is exempt", "svc.go", "0 finding", "INVALID", """
package x

//nolint:gosec
func Run() {}
""")

    check_case("K13 a step marker is INVALID", "svc.go", "STEP_MARKER", "", """
package x

func Run() {
\t// 1. parse the input
\tparse()
}
""")

    check_case("K14 a banner outside a _test.go file is INVALID", "svc.go",
               "BANNER_OUTSIDE_TEST", "", """
package x

// --- Setup ---
func Run() {}
""")

    check_case("K15 a malformed marker is INVALID", "svc.go", "MALFORMED_MARKER", "", """
package x

// TODO:
func Run() {}
""")

    check_case("K16 a // inside a string literal is not a comment", "svc.go", "0 finding",
               "INVALID", """
package x

func Run() {
\ts := "http://example.com/path"
\t_ = s
}
""")

    check_case("K21 a file-header numbered list is exempt from STEP_MARKER", "script.sql",
               "0 finding", "STEP_MARKER", """
-- script.sql - does the thing.
--
-- Steps, in order:
--   1. parse the input
--   2. run the thing
SELECT 1;
""")

    check_case("K22 a file-header comment run is exempt from STACKED", "script.sql",
               "0 finding", "STACKED", """
-- script.sql - does the thing.
--
-- More explanation spread across several adjacent comment lines.
SELECT 1;
""")

    check_case("K23 a step marker outside the file header is still INVALID", "script.sql",
               "STEP_MARKER", "", """
SELECT 1;

-- 1. parse the input
SELECT 2;
""")

    check_case("K24 a banner outside a _test.go file is exempt for non-Go files",
               "script.sql", "0 finding", "BANNER_OUTSIDE_TEST", """
SELECT 1;

-- --- Setup ---
SELECT 2;
""")

    padding = "".join(
        "-- line %02d of padding narrative disguised as a file header\n" % n
        for n in range(1, 31)
    )
    check_case("K25 a header run past HEADER_MAX_LINES loses its exemption", "script.sql",
               "STACKED", "",
               padding
               + "-- line 31, past the cap, adjacent to line 30, should trigger STACKED\n"
               + "SELECT 1;\n")


def check_git_guard() -> None:
    emit("\ngit guard\n\n")
    FIXTURE_MAIN = FIXTURES["main"]
    FIXTURE_FEAT = FIXTURES["feat"]
    SOMEDIR = FIXTURES["somedir"]
    OUTSIDE_REPO = FIXTURES["outside"]
    OUTSIDE_REPO_2 = FIXTURES["outside2"]
    COMMENTS_COPY = FIXTURES["comments_copy"]
    COMMENTS_ALIAS = FIXTURES["comments_alias"]
    bash_case_at(
        FIXTURE_MAIN,
        'G1a committing directly on a protected branch is blocked',
        'deny',
        'git commit -m "wip"',
        'create a branch first',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        'G1b committing on a feature branch is allowed',
        'allow',
        'git commit -m "wip"',
    )
    bash_case('G2  git log is allowed', 'allow', 'git log --oneline -20')
    bash_case('G3  git restore is blocked', 'deny', 'git restore src/main.go', 'git restore')
    bash_case(
        'G4  git checkout -- is blocked', 'deny', 'git checkout -- src/main.go', 'git checkout',
    )
    bash_case('G5  echo of a git string is not a match', 'allow', 'echo "git commit -m x"')
    bash_case(
        'G6  sh -c wrapper is unwrapped',
        'deny',
        'sh -c "git push origin main"',
        'open a pull request instead',
    )
    bash_case(
        'G7  sed -i is blocked',
        'deny',
        "sed -i '' 's/a/b/' main.go",
        'bypassing the comment guard',
    )
    bash_case(
        'G8  redirect into a code file is blocked',
        'deny',
        'cat > handler.go',
        'bypasses the comment guard',
    )
    bash_case('G9  chained read-only git is allowed', 'allow', 'git status && git diff --stat')
    bash_case_cwd(
        FIXTURE_MAIN,
        'G10 chained commit on a protected branch is caught',
        'deny',
        'git diff && git commit -m x',
        'create a branch first',
    )
    bash_case('G11 bare git branch lists and is allowed', 'allow', 'git branch -a')
    bash_case(
        'G12a creating a conventionally-named branch is allowed', 'allow', 'git branch feat/x',
    )
    bash_case(
        'G12b creating a badly-named branch is blocked',
        'deny',
        'git branch feature/x',
        'kebab-case',
    )
    bash_case(
        'G12c naming a protected branch is blocked', 'deny', 'git branch main', 'kebab-case',
    )
    bash_case('G13a bare git stash (push-equivalent) is allowed', 'allow', 'git stash')
    bash_case(
        'G13b git stash drop is blocked', 'deny', 'git stash drop', 'discards saved state',
    )
    bash_case('G14 git fetch is allowed', 'allow', 'git fetch origin')
    bash_case('G15 git reset --hard is blocked', 'deny', 'git reset --hard HEAD~1', 'git reset')
    bash_case('G16 unrelated commands pass', 'allow', 'go test ./... && npm run build')
    bash_case('G17 redirect to a non-code file passes', 'allow', 'go test ./... > /tmp/out.txt')
    bash_case('G19 git stash list is allowed', 'allow', 'git stash list')
    bash_case(
        'G20 git config set is blocked', 'deny', 'git config user.email a@b.c', 'sets a value',
    )
    bash_case('G21 git config get is allowed', 'allow', 'git config --get user.email')
    bash_case(
        'G22 perl -i is blocked',
        'deny',
        "perl -pi -e 's/a/b/' main.go",
        'bypassing the comment guard',
    )
    bash_case(
        'G23 append redirect to code is blocked',
        'deny',
        'echo x >> util.ts',
        'bypasses the comment guard',
    )
    bash_case('G18 git clean is blocked', 'deny', 'git clean -fdx', 'git clean')
    bash_case('G24 an ASCII arrow is not a redirect', 'allow', 'echo "step 1 -> main.go done"')
    bash_case(
        'G25 the word tee in a string is not tee',
        'allow',
        'echo "redirect/tee cases" && ls scripts/test_hooks.py',
    )
    bash_case(
        'G26 real tee into a code file is blocked',
        'deny',
        'go test ./... | tee results.go',
        'tee writing to',
    )
    bash_case('G27 a quoted redirect is data', 'allow', 'echo "write it with > handler.go"')
    bash_case(
        'G28 a heredoc body is data',
        'allow',
        'python3 - <<\'PY\'\nprint("emit > handler.go here")\nPY',
    )
    bash_case(
        'G29 a redirect on the heredoc opener still blocks',
        'deny',
        "cat <<'EOF' > handler.go\npackage main\nEOF",
        'bypasses the comment guard',
    )
    bash_case('G30 numeric fd redirect is not a code path', 'allow', 'go build ./... 2>&1')
    bash_case(
        'G31 cp over a code path is blocked',
        'deny',
        'cp /tmp/staged.go handler.go',
        'bypasses the comment guard',
    )
    bash_case(
        'G32 mv over a code path is blocked',
        'deny',
        'mv /tmp/staged.go handler.go',
        'bypasses the comment guard',
    )
    bash_case(
        'G99 redirect into a .json file is blocked',
        'deny',
        'echo x > claude-code/settings.json',
        'bypasses the comment guard',
    )
    bash_case(
        'G100 redirect into a .md file is blocked',
        'deny',
        'echo x > BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G101 a dollar-paren command-substitution redirect target is blocked',
        'deny',
        'echo bad >$(echo BACKLOG.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G102 a backtick command-substitution tee target is blocked',
        'deny',
        'tee `echo settings.json`',
        'bypasses the comment guard',
    )
    bash_case(
        'G103 a dollar-paren command-substitution tee target is blocked',
        'deny',
        'tee $(echo settings.json)',
        'bypasses the comment guard',
    )
    bash_case(
        'G104 a double-quoted redirect target is blocked',
        'deny',
        'echo bad > "BACKLOG.md"',
        'bypasses the comment guard',
    )
    bash_case(
        'G105 a single-quoted redirect target is blocked',
        'deny',
        "echo bad > 'BACKLOG.md'",
        'bypasses the comment guard',
    )
    bash_case(
        'G106 sed -i merely named inside a quoted string is allowed',
        'allow',
        "echo 'sed -i is mentioned here'",
    )
    bash_case(
        'G107 a real unquoted sed -i is still blocked',
        'deny',
        'sed -i -e s/a/b/ file',
        'bypassing the comment guard',
    )
    bash_case(
        'G108 a substitution concatenated after literal text in a redirect target is blocked',
        'deny',
        'echo bad > pre$(echo _BACKLOG.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G109 a substitution concatenated after literal text in a tee target is blocked',
        'deny',
        'tee pre$(echo _BACKLOG.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G110 a quoted concatenated-substitution redirect target is blocked',
        'deny',
        'echo bad > "pre$(echo BACKLOG.md)"',
        'bypasses the comment guard',
    )
    bash_case(
        'G111 a nested dollar-paren substitution redirect target is blocked',
        'deny',
        'echo bad > $(echo $(echo BACKLOG.md))',
        'bypasses the comment guard',
    )
    bash_case(
        'G112 a nested dollar-paren substitution tee target is blocked',
        'deny',
        'tee $(echo $(echo BACKLOG.md))',
        'bypasses the comment guard',
    )
    bash_case(
        'G113 a mid-word quote split rejoining a guarded filename in tee is blocked',
        'deny',
        'tee Docker"file"',
        'bypasses the comment guard',
    )
    bash_case(
        'G114 a mid-word quote split rejoining a guarded filename in a redirect is blocked',
        'deny',
        'echo x > Make"file"',
        'bypasses the comment guard',
    )
    bash_case(
        'G115 a dollar-paren command-substitution cp target is blocked',
        'deny',
        'cp /tmp/a.txt $(echo handler.go)',
        'bypasses the comment guard',
    )
    bash_case(
        'G116 a dollar-paren command-substitution cp target naming a doc path is blocked',
        'deny',
        'cp /tmp/a.txt $(echo BACKLOG.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G117 a dollar-paren command-substitution mv target is blocked',
        'deny',
        'mv /tmp/a.txt $(echo settings.json)',
        'bypasses the comment guard',
    )
    bash_case(
        'G118 a redirect target wrapped in a bare subshell is blocked',
        'deny',
        '(echo bad > BACKLOG.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G119 a tee target wrapped in a bare subshell is blocked',
        'deny',
        '(tee BACKLOG.md <<< bad)',
        'bypasses the comment guard',
    )
    bash_case(
        'G120 a redirect target with a literal unquoted paren keeps its extension',
        'deny',
        'echo hi > BACKLOG(x).md',
        'bypasses the comment guard',
    )
    bash_case(
        'G121 a redirect target with a literal unquoted paren keeps its extension (json)',
        'deny',
        'cat x > settings(1).json',
        'bypasses the comment guard',
    )
    bash_case(
        'G122 a tee target wrapped in a bare subshell with no space before the paren is blocked',
        'deny',
        '(tee AGENTS.md) <<< "malicious content"',
        'bypasses the comment guard',
    )
    bash_case(
        'G123 a cp target wrapped in a bare subshell is blocked',
        'deny',
        '(cp malicious.txt AGENTS.md)',
        'bypasses the comment guard',
    )
    bash_case(
        'G124 a mv target wrapped in a bare subshell is blocked',
        'deny',
        '(mv malicious.txt AGENTS.md)',
        'bypasses the comment guard',
    )
    bash_case('G33 cp between non-code paths is allowed', 'allow', 'cp /tmp/a.txt /tmp/b.txt')
    bash_case('G34 mv of a directory listing is allowed', 'allow', 'mv build/ dist/')
    bash_case('G35 time git rebase is caught', 'deny', 'time git rebase main', 'git rebase')
    bash_case(
        'G36 command git rebase is caught', 'deny', 'command git rebase main', 'git rebase',
    )
    bash_case(
        'G37 xargs git rebase is caught', 'deny', 'xargs -I{} git rebase main', 'git rebase',
    )
    bash_case(
        'G38 nohup git push is caught', 'deny', 'nohup git push origin main &', 'git push',
    )
    bash_case(
        'G39 eval of a git string is caught', 'deny', 'eval "git rebase main"', 'git rebase',
    )
    bash_case('G40 time go test is allowed', 'allow', 'time go test ./...')
    bash_case(
        "G41 an unlisted long value-flag on a passthrough wrapper doesn't hide the wrapped command",
        'deny',
        'time --output logfile.py git rebase main',
        'git rebase',
    )
    bash_case(
        'G42 mv into a trailing-slash directory destination is blocked',
        'deny',
        'mv payload.py somedir/',
        'bypasses the comment guard',
    )
    bash_case(
        'G43 cp into an existing bare-name directory destination is blocked',
        'deny',
        'cp payload.py ' + SOMEDIR,
        'bypasses the comment guard',
    )
    bash_case(
        'G44 cp -t names the real target out of position',
        'deny',
        'cp -t module.py staged.txt',
        'bypasses the comment guard',
    )
    bash_case(
        'G45 cp -t DIR still checks the sources being placed',
        'deny',
        'cp -t somedir payload.go',
        'bypasses the comment guard',
    )
    bash_case(
        'G46 mv -t DIR/ still checks the sources being placed',
        'deny',
        'mv -t assets/ handler.go',
        'bypasses the comment guard',
    )
    bash_case(
        'G47 mv into a directory created earlier in the same command',
        'deny',
        'mkdir -p brandnewdir && mv payload.go brandnewdir',
        'bypasses the comment guard',
    )
    bash_case(
        'G48 git tag creating an annotated tag is allowed',
        'allow',
        'git tag -a v1.2.3 abc123 -m "release"',
    )
    bash_case(
        'G49 git tag creating a lightweight tag is allowed', 'allow', 'git tag v1.2.3 abc123',
    )
    bash_case(
        'G50 git tag -d is blocked', 'deny', 'git tag -d v1.2.3', 'changes repository state',
    )
    bash_case(
        'G51 git tag -f is blocked',
        'deny',
        'git tag -f v1.2.3 abc123',
        'changes repository state',
    )
    bash_case(
        'G52 git switch -c with a conventional name is allowed',
        'allow',
        'git switch -c feat/redesign',
    )
    bash_case(
        'G53 git switch -c with a bad name is blocked',
        'deny',
        'git switch -c badname',
        'kebab-case',
    )
    bash_case(
        'G54 git switch -c naming a protected branch is blocked',
        'deny',
        'git switch -c main',
        'kebab-case',
    )
    bash_case('G55 git switch to an existing branch is allowed', 'allow', 'git switch main')
    bash_case(
        'G56 git switch --detach is blocked', 'deny', 'git switch --detach HEAD', 'is denied',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        'G57 git push on a feature branch is allowed',
        'allow',
        'git push -u origin feat/test-branch',
    )
    bash_case(
        'G58 git push to main is blocked',
        'deny',
        'git push origin main',
        'open a pull request instead',
    )
    bash_case(
        'G59 git push --force is blocked',
        'deny',
        'git push --force origin feat/x',
        'rewrites published history',
    )
    bash_case('G60 git add -A is allowed', 'allow', 'git add -A')
    bash_case('G61 git add -p is blocked', 'deny', 'git add -p', 'interactive')
    bash_case(
        'G62 a commit with a co-author trailer is blocked',
        'deny',
        'git commit -m "fix: x\n\nCo-Authored-By: bot <b@b.com>"',
        'trailer',
    )
    bash_case(
        'G63 git commit --amend is blocked',
        'deny',
        'git commit --amend -m x',
        'rewrites a commit',
    )
    bash_case('G64 gh pr create is allowed', 'allow', 'gh pr create --title x --body y')
    bash_case('G65 gh pr merge is blocked', 'deny', 'gh pr merge 5', 'is denied')
    bash_case('G66 gh label create is blocked', 'deny', 'gh label create foo', 'is denied')
    bash_case('G67 gh label list is allowed', 'allow', 'gh label list')
    bash_case(
        'G68 gh api with a write flag is blocked',
        'deny',
        'gh api repos/x/y -X DELETE',
        'write requests are denied',
    )
    bash_case('G69 gh api read is allowed', 'allow', 'gh api repos/x/y/dependabot/alerts')
    bash_case('G70 gh release create is blocked', 'deny', 'gh release create v1.0', 'is denied')
    bash_case_cwd(
        FIXTURE_FEAT,
        'G71 merging the protected base into a feature branch is allowed',
        'allow',
        'git merge origin/main',
    )
    bash_case_cwd(
        FIXTURE_FEAT, 'G72 merging the bare base name is allowed', 'allow', 'git merge main',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        "G72b a trailing stderr redirect doesn't miscount the merge target",
        'allow',
        'git merge main 2>&1',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        "G72c a trailing stdout redirect doesn't miscount the merge target",
        'allow',
        'git merge main > out.txt',
    )
    bash_case_cwd(
        FIXTURE_MAIN,
        'G73 merging into a protected branch is blocked',
        'deny',
        'git merge feat/test-branch',
        'landing into it',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        'G74 merging a non-base branch is blocked',
        'deny',
        'git merge some-other-branch',
        'protected base branch',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        'G75 merge -X ours is blocked',
        'deny',
        'git merge origin/main -X ours',
        'without a visible conflict',
    )
    bash_case_cwd(FIXTURE_FEAT, 'G76 merge --abort is allowed', 'allow', 'git merge --abort')
    bash_case(
        'G77 git rebase is blocked', 'deny', 'git rebase main', 'changes repository state',
    )
    bash_case(
        'G78 git pull without --ff-only is blocked',
        'deny',
        'git pull origin main',
        'only with --ff-only',
    )
    bash_case('G78b git pull --ff-only is allowed', 'allow', 'git pull --ff-only origin main')
    bash_case(
        "G88 time --output naming a shell-wrapper value doesn't hide the real wrapped command",
        'deny',
        'time --output sh git rebase main',
        'git rebase',
    )
    bash_case(
        "G89 xargs --delimiter naming a monitored command as its value doesn't hide the real wrapped command",
        'deny',
        'xargs --delimiter git git push --force origin main',
        'git push --force',
    )
    bash_case(
        'G90 a single-quoted grep pattern with a backtick alternation naming git log is allowed',
        'allow',
        "grep -n -E 'foo`|git log`' file.txt",
    )
    bash_case(
        'G91 a quote-escaped single-quoted pattern with a backtick and git log is allowed',
        'allow',
        'grep -n -E \'a\'"\'"\'b`|git log`\' file.txt',
    )
    bash_case(
        'G92 a single-quoted grep pattern with backtick git log text still lets a real segment after it deny',
        'deny',
        "grep -n 'note: `git log`' file.txt; git push origin main",
        'open a pull request instead',
    )
    bash_case(
        "G93 a quote inside a shell comment doesn't swallow the next line's real command",
        'deny',
        "git status # '\ngit push --force",
        'rewrites published history',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        "G94 a # inside a quoted commit message isn't misread as a comment opener",
        'allow',
        'git commit -m "see issue #42"',
    )
    bash_case(
        'G95 a double-quoted dollar-paren substitution running git push --force is denied',
        'deny',
        'echo "note: $(git push --force) is not real here"',
        'rewrites published history',
    )
    bash_case(
        'G96 a real unquoted backtick substitution running git push --force is denied',
        'deny',
        'echo `git push --force`',
        'rewrites published history',
    )
    bash_case(
        'G97 a real unquoted dollar-paren substitution running git push --force is denied',
        'deny',
        'echo $(git push --force)',
        'rewrites published history',
    )
    bash_case(
        'G98 old-style backslash-escaped nested backticks hiding git push --force is denied',
        'deny',
        'echo `echo \\`git push --force\\`` ',
        'rewrites published history',
    )
    bash_case(
        'G125 a >1 fd redirect into a guarded path is blocked',
        'deny',
        'echo bad 1>BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G126 a 2> fd redirect into a guarded path is blocked',
        'deny',
        'echo bad 2>BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G127 a 9> fd redirect into a guarded path is blocked',
        'deny',
        'echo bad 9>BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G128 an &> redirect into a guarded path is blocked',
        'deny',
        'echo bad &>BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G129 a double-quoted -i flag still triggers sed -i detection',
        'deny',
        'sed "-i" -e s/a/b/ file',
        'bypassing the comment guard',
    )
    bash_case(
        'G130 a single-quoted -i flag still triggers sed -i detection',
        'deny',
        "sed '-i' -e s/a/b/ file",
        'bypassing the comment guard',
    )
    bash_case(
        'G131 a quoted -i flag still triggers perl -i detection',
        'deny',
        'perl "-i" -pe s/a/b/ file',
        'bypassing the comment guard',
    )
    bash_case(
        'G134 a single-quote split inside the -i flag still triggers sed -i detection',
        'deny',
        "sed -'i' -e s/a/b/ BACKLOG.md",
        'bypassing the comment guard',
    )
    bash_case(
        'G135 a double-quote split inside the -i flag still triggers sed -i detection',
        'deny',
        'sed -"i" -e s/a/b/ BACKLOG.md',
        'bypassing the comment guard',
    )
    bash_case_cwd(
        OUTSIDE_REPO,
        'G132 a guarded-extension write outside any repo is allowed',
        'allow',
        'echo x > BACKLOG.md',
    )
    bash_case_cwd(
        FIXTURE_MAIN,
        'G133 a guarded-extension write inside the repo still denies',
        'deny',
        'echo x > BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case_cwd(
        OUTSIDE_REPO,
        'G137 cwd outside any repo does not exempt an absolute target that lands inside a real repo',
        'deny',
        'cp malicious.txt ' + FIXTURE_MAIN + '/BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case_cwd(
        OUTSIDE_REPO_2,
        'G138 cwd outside any repo and an absolute target outside any repo is still allowed',
        'allow',
        'echo x > ' + OUTSIDE_REPO + '/BACKLOG.md',
    )
    deep_chain = "$(" * DEEP_NESTING_DEPTH + "true" + ")" * DEEP_NESTING_DEPTH
    bash_case(
        'G139 a command-substitution chain nested thousands deep cannot recursion-exhaust past a real guarded write',
        'deny',
        deep_chain + '; sed -i s/x/y/ BACKLOG.md',
        'bypassing the comment guard',
    )
    deep_target = 'BACKLOG.md'
    for _ in range(DEEP_NESTING_DEPTH):
        deep_target = '$(echo %s)' % deep_target
    bash_case(
        'G140 a redirect target word nested thousands deep in $(...) cannot recursion-exhaust into a fail-open allow',
        'deny',
        'echo bad > ' + deep_target,
        'could not be safely analyzed',
    )
    crash_case('G141 crash: echo hello is still allowed', 'allow', 'echo hello')
    crash_case(
        'G142 crash: git push origin main is still denied',
        'deny',
        'git push origin main',
        'git push',
    )
    crash_case('G143 crash: rm -rf / is still denied', 'deny', 'rm -rf /', 'rm -rf')
    off_case(
        'G79 off: push is blocked',
        'deny',
        'git push -u origin feat/x',
        'changes repository state',
    )
    off_case(
        'G80 off: commit is blocked', 'deny', 'git commit -m x', 'changes repository state',
    )
    off_case(
        'G81 off: switch -c is blocked',
        'deny',
        'git switch -c feat/x',
        'changes repository state',
    )
    off_case(
        'G82 off: branch creation is blocked', 'deny', 'git branch feat/x', 'creates a branch',
    )
    off_case('G83 off: stash push is blocked', 'deny', 'git stash', 'changes repository state')
    off_case(
        'G84 off: merge --ff-only is still blocked',
        'deny',
        'git merge --ff-only origin/main',
        'changes repository state',
    )
    off_case('G85 off: git add is blocked', 'deny', 'git add .', 'changes repository state')
    off_case('G86 off: read-only git still works', 'allow', 'git log --oneline -5')
    off_case('G87 off: gh pr merge stays blocked', 'deny', 'gh pr merge 12', 'is denied')
    bash_case_agent(
        'G144 reviewer tee to a non-BACKLOG.md file is denied',
        'deny',
        'reviewer',
        'tee notes.txt',
        'read-only git',
    )
    bash_case_agent(
        'G145 reviewer redirect to an extensionless file is denied',
        'deny',
        'reviewer',
        'echo x > release-notes',
        'bypasses the comment guard',
    )
    bash_case_agent(
        'G146 reviewer tee to BACKLOG.md is denied',
        'deny',
        'reviewer',
        'tee BACKLOG.md',
        'read-only git',
    )
    bash_case_agent(
        'G147 reviewer redirect to BACKLOG.md is denied',
        'deny',
        'reviewer',
        'echo x > BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case(
        'G148 a non-reviewer tee to a non-guarded extensionless file is unaffected (still allowed)',
        'allow',
        'tee notes.txt',
    )
    bash_case(
        'G149 a non-reviewer tee to BACKLOG.md is unaffected (still denied)',
        'deny',
        'tee BACKLOG.md',
        'bypasses the comment guard',
    )
    bash_case_agent_at(
        OUTSIDE_REPO,
        'G150 reviewer tee to BACKLOG.md from a cwd outside any git repo is denied',
        'deny',
        'reviewer',
        'tee BACKLOG.md',
        'read-only git',
    )
    bash_case(
        'G151 a backslash-escaped backtick inside a $()-substitution fed through eval hides git push --force',
        'deny',
        'eval "$(echo \\`git push --force\\`)"',
        'rewrites published history',
    )
    bash_case(
        'G152 a single-quoted sh -c argument containing a double-quoted dollar-paren substitution, nested 2 levels, hides git push --force',
        'deny',
        'echo "$(sh -c \'echo "$(git push --force)"\')"',
        'rewrites published history',
    )
    bash_case(
        'G153 a backslash-escaped backtick inside a $()-substitution with no eval/-c re-parse is inert text and allowed',
        'allow',
        'echo "$(echo \\`echo git push --force\\` is dangerous)"',
    )
    bash_case(
        "G154 command eval indirection re-parses a $()-substitution's escaped backtick, same as a bare eval",
        'deny',
        'command eval "$(echo x\\`git push origin main\\`y)"',
        'is denied',
    )
    bash_case(
        'G155 a # comment line before an eval on the next line still re-parses its escaped backtick',
        'deny',
        'echo hi # comment\neval "$(echo \\`git push --force\\`)"',
        'rewrites published history',
    )
    bash_case(
        "G164 command -v eval never re-parses its $()-substitution's escaped backtick, so it stays inert text and is allowed",
        'allow',
        'command -v eval "$(echo \\`git push --force\\`)"',
    )
    bash_case(
        "G165 command -V eval never re-parses its $()-substitution's escaped backtick, so it stays inert text and is allowed",
        'allow',
        'command -V eval "$(echo \\`git push --force\\`)"',
    )
    bash_case_agent(
        'G156 reviewer python3 comments.py apply is denied',
        'deny',
        'reviewer',
        'python3 ~/.claude/hooks/comments.py apply',
        'read-only git',
    )
    bash_case(
        'G157 a non-reviewer python3 comments.py apply is unaffected (still allowed)',
        'allow',
        'python3 ~/.claude/hooks/comments.py apply',
    )
    bash_case_agent(
        'G158 reviewer python3 -m pytest is denied',
        'deny',
        'reviewer',
        'python3 -m pytest',
        'read-only git',
    )
    bash_case_agent(
        'G159 reviewer direct comments.py apply via a relative path (no interpreter prefix) is denied',
        'deny',
        'reviewer',
        'claude-code/hooks/comments.py apply',
        'read-only git',
    )
    bash_case_agent_at(
        HOOKS,
        'G160 reviewer bare comments.py apply, cwd inside hooks/, is denied',
        'deny',
        'reviewer',
        'comments.py apply',
        'read-only git',
    )
    bash_case_agent(
        'G161 reviewer cat comments.py piped into python3 - apply (script read off stdin) is denied',
        'deny',
        'reviewer',
        'cat ~/.claude/hooks/comments.py | python3 - apply',
        'read-only git',
    )
    bash_case_agent(
        'G162 reviewer a same-content copy of comments.py under an unrelated basename is denied',
        'deny',
        'reviewer',
        COMMENTS_COPY + ' apply',
        'read-only git',
    )
    bash_case_agent(
        'G163 reviewer a same-file alias under a different-case basename is denied',
        'deny',
        'reviewer',
        'python3 ' + COMMENTS_ALIAS + ' apply',
        'read-only git',
    )
    bash_case_agent(
        'G166 reviewer env python3 comments.py apply is denied, real env has no -c flag to gate the recursion on',
        'deny',
        'reviewer',
        'env python3 ~/.claude/hooks/comments.py apply',
        'read-only git',
    )
    bash_case_agent_at(
        HOOKS, 'G179 reviewer git log is allowed', 'allow', 'reviewer', 'git log',
    )
    bash_case_agent_at(
        HOOKS, 'G180 reviewer git diff is allowed', 'allow', 'reviewer', 'git diff',
    )
    bash_case_agent_at(
        HOOKS, 'G181 reviewer git status is allowed', 'allow', 'reviewer', 'git status',
    )
    bash_case_agent_at(
        HOOKS, 'G182 reviewer git show is allowed', 'allow', 'reviewer', 'git show',
    )
    bash_case_agent_at(
        HOOKS,
        'G183 reviewer git blame on a tracked file is allowed',
        'allow',
        'reviewer',
        'git blame -- git_guard.py',
    )
    bash_case_agent_at(
        HOOKS, 'G184 reviewer git ls-files is allowed', 'allow', 'reviewer', 'git ls-files',
    )
    bash_case_agent_at(
        FIXTURE_MAIN,
        'G185 reviewer git commit -m is still blocked by the pre-existing mechanism',
        'deny',
        'reviewer',
        'git commit -m "x"',
        'is denied',
    )
    bash_case_agent(
        'G186 reviewer a never-before-seen interpreter is denied by the generic gate',
        'deny',
        'reviewer',
        'ruby -e "File.write(1,1)"',
        'read-only git',
    )
    bash_case_agent(
        'G187 reviewer gh pr view is denied even though it looks read-only',
        'deny',
        'reviewer',
        'gh pr view',
        'read-only git',
    )
    bash_case(
        'G188 a non-reviewer python3 -m pytest is unaffected (still allowed)',
        'allow',
        'python3 -m pytest',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G189 reviewer git add && git commit on a non-protected branch is still denied',
        'deny',
        'reviewer',
        'git add -A && git commit -m x',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G190 reviewer git push on a non-protected branch is still denied',
        'deny',
        'reviewer',
        'git push',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G191 reviewer git branch with a validly-named branch is still denied',
        'deny',
        'reviewer',
        'git branch feat/some-valid-name',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G192 reviewer bare git stash on a non-protected branch is still denied',
        'deny',
        'reviewer',
        'git stash',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G193 reviewer git switch on a non-protected branch is still denied',
        'deny',
        'reviewer',
        'git switch some-other-branch',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G194 reviewer git diff --output writes to a file and is denied',
        'deny',
        'reviewer',
        'git diff --output=BACKLOG.md',
        'has no legitimate write path',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G195 reviewer git log --output writes to a file and is denied',
        'deny',
        'reviewer',
        'git log --output=x',
        'has no legitimate write path',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G196 reviewer git show --output writes to a file and is denied',
        'deny',
        'reviewer',
        'git show --output=x',
        'has no legitimate write path',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        "G197 reviewer git status stays allowed (subcommand allowlist isn't over-restrictive)",
        'allow',
        'reviewer',
        'git status',
    )
    bash_case_cwd(
        FIXTURE_FEAT,
        'G198 a non-reviewer git commit on a non-protected branch is unaffected (still allowed)',
        'allow',
        'git commit -m x',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G199 reviewer git diff --output <file>, two-token form, is denied',
        'deny',
        'reviewer',
        'git diff --output BACKLOG.md',
        'has no legitimate write path',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G200 reviewer GIT_EXTERNAL_DIFF=... git diff executes an arbitrary command and is denied',
        'deny',
        'reviewer',
        'GIT_EXTERNAL_DIFF="touch pwn" git diff',
        'VAR=value',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        "G201 reviewer git -c core.pager=x diff redirects git's pager and is denied",
        'deny',
        'reviewer',
        'git -c core.pager=x diff',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G202 reviewer git -C /tmp log hides the subcommand behind a global flag and is denied',
        'deny',
        'reviewer',
        'git -C /tmp log',
        'read-only git',
    )
    bash_case_agent_at(
        FIXTURE_FEAT,
        'G203 reviewer git --exec-path=/tmp/evil diff points git at a malicious binary and is denied',
        'deny',
        'reviewer',
        'git --exec-path=/tmp/evil diff',
        'read-only git',
    )
    bash_case(
        'G204 a non-reviewer GIT_EXTERNAL_DIFF=... git diff is unaffected (still allowed)',
        'allow',
        'GIT_EXTERNAL_DIFF="touch pwn" git diff',
    )
    bash_case(
        'G205 a non-reviewer git -c core.pager=x diff is unaffected (still allowed)',
        'allow',
        'git -c core.pager=x diff',
    )
    bash_case(
        'G167 a non-reviewer env python3 comments.py apply is unaffected (still allowed)',
        'allow',
        'env python3 ~/.claude/hooks/comments.py apply',
    )
    bash_case(
        'G168 env skips leading -i and VAR=val tokens to reach a wrapped git push --force',
        'deny',
        'env -i FOO=bar git push origin --force',
        'rewrites published history',
    )
    bash_case(
        'G169 a harmless env-wrapped command is unaffected (still allowed)',
        'allow',
        'env echo hello',
    )
    bash_case(
        'G170 env -u NAME (a real env value flag outside -i/VAR=val) is skipped to reach a wrapped git push --force',
        'deny',
        'env -u PATH git push --force',
        'rewrites published history',
    )
    bash_case(
        'G171 env -C DIR (a real env value flag outside -i/VAR=val) is skipped to reach a wrapped git push --force',
        'deny',
        'env -C /tmp git push --force',
        'rewrites published history',
    )
    bash_case(
        'G172 env --ignore-environment (a real env boolean flag outside -i/VAR=val) is skipped to reach a wrapped git push --force',
        'deny',
        'env --ignore-environment git push --force',
        'rewrites published history',
    )
    bash_case(
        'G173 env sh -c "..." preserves the wrapped script\'s quoting through the recursive scan and is denied',
        'deny',
        'env sh -c "git push origin main --force"',
        'rewrites published history',
    )
    bash_case(
        'G174 time sh -c "..." preserves the wrapped script\'s quoting through the recursive scan and is denied',
        'deny',
        'time sh -c "git push --force"',
        'rewrites published history',
    )
    bash_case(
        'G175 nohup sh -c "..." preserves the wrapped script\'s quoting through the recursive scan and is denied',
        'deny',
        'nohup sh -c "git push --force"',
        'rewrites published history',
    )
    bash_case(
        'G176 env -S "..." (separated form) recurses into its split-string argument and is denied',
        'deny',
        'env -S "git push --force"',
        'rewrites published history',
    )
    bash_case(
        'G177 env -Sstring (attached form) recurses into its split-string argument and is denied',
        'deny',
        'env -Sgit\\ push\\ --force',
        'rewrites published history',
    )
    bash_case(
        'G178 env --split-string="..." (GNU long form) recurses into its split-string argument and is denied',
        'deny',
        'env --split-string="git push --force"',
        'rewrites published history',
    )


def check_auto_format() -> None:
    emit("\nauto format\n\n")
    fmt = os.path.join(WORKDIR[0], "fmt")
    emptybin = os.path.join(WORKDIR[0], "emptybin")
    os.makedirs(fmt)
    os.makedirs(emptybin)

    write_text(os.path.join(fmt, "main.go"), "package main\n\nfunc  main() {}\n")
    if IS_WINDOWS:
        skip_case(
            "F1  gofmt reformats a .go file when gofmt is present",
            'shutil.which("gofmt") returns None on the Windows Git Bash runner despite '
            "setup-go adding it to PATH",
        )
    else:
        auto_format_case("F1  gofmt reformats a .go file when gofmt is present", "yes",
                         os.path.join(fmt, "main.go"), "Write")

    write_text(os.path.join(fmt, "nogofmt.go"), "package main\n\nfunc  main() {}\n")
    auto_format_case("F2  no-ops silently when gofmt isn't on PATH", "no",
                     os.path.join(fmt, "nogofmt.go"), "Write", emptybin)

    write_text(os.path.join(fmt, "widget.ts"), "const   x = 1;\n")
    auto_format_case("F3  no-ops silently when prettier isn't on PATH", "no",
                     os.path.join(fmt, "widget.ts"), "Edit", emptybin)

    write_text(os.path.join(fmt, "notes.txt"), "not a real config file\n")
    auto_format_case("F4  an extension with no formatter is left alone", "no",
                     os.path.join(fmt, "notes.txt"), "Write")

    expect("F5  MultiEdit is not in scope and never blocks", "allow",
           '{"tool_name":"MultiEdit","tool_input":{"file_path":"%s"}}'
           % os.path.join(fmt, "main.go"))
    expect("F6  a missing file_path never blocks", "allow",
           '{"tool_name":"Write","tool_input":{}}')
    expect("F7  a malformed payload fails open, not closed", "allow",
           '{"tool_name":"Write","tool_input":')

    bash_case_agent("R1  reviewer echo x >> BACKLOG.md is denied", "deny", "reviewer",
                    "echo x >> BACKLOG.md", "bypasses the comment guard")
    bash_case("R2  non-reviewer echo x >> BACKLOG.md is unaffected (still denied by the "
              "comment guard)", "deny", "echo x >> BACKLOG.md",
              "bypasses the comment guard")


def check_comments_apply() -> None:
    emit("\ncomments apply\n\n")

    validator_case(
        "V1  a BANNER outside a _test.go file is dropped and a valid plain WHY proposal "
        "is spliced",
        '[{"file":"svc.go","line":4,"template_type":"WHY","text":"// seeds the retry '
        'counter"},{"file":"svc.go","line":3,"template_type":"BANNER","text":"// --- '
        'Setup ---"}]',
        "seeds the retry counter", "only allowed in a _test.go file",
        "seeds the retry counter")

    validator_case(
        "V2  a proposal whose file escapes repo_root via ../ segments is dropped, not "
        "spliced",
        '[{"file":"../../../../../../etc/escape-me.go","line":1,"template_type":"WHY",'
        '"text":"// WHY: should never land outside the repo"}]',
        "", "outside repo root")

    validator_own_fixture_case(
        "V4  a plain WHY body over the 120-char cap is dropped, not spliced",
        '[{"file":"svc.go","line":4,"template_type":"WHY","text":"// this sentence is '
        'deliberately padded well past the one hundred and twenty character cap so the '
        'validator has to drop it outright"}]',
        "", "over the 120-char cap")

    validator_own_fixture_case(
        "V5  a second proposal within 2 lines of an already-spliced one is dropped as a "
        "stack",
        '[{"file":"svc.go","line":4,"template_type":"WHY","text":"// first comment at '
        'this site"},{"file":"svc.go","line":5,"template_type":"NOTE","text":"// NOTE: '
        'second comment stacked too close"}]',
        "second comment stacked too close", "one comment per site, not a stack")

    validator_own_fixture_case(
        "V6  a WHY proposal with the deprecated WHY: marker prefix is dropped",
        '[{"file":"svc.go","line":4,"template_type":"WHY","text":"// WHY: seeds the '
        'retry counter"}]',
        "", "plain sentence with no marker prefix")

    validator_own_fixture_case(
        "V7  a TODO proposal with the deprecated TODO(user) form is dropped",
        '[{"file":"svc.go","line":4,"template_type":"TODO","text":"// TODO(rad): fix '
        'this later"}]',
        "", "does not match the TODO template shape")

    validator_own_fixture_case(
        "V8  a DOC comment lands name-first on an exported func and is not flagged as an "
        "echo",
        '[{"file":"svc.go","line":3,"template_type":"DOC","text":"// Run seeds the retry '
        'counter and starts the loop."}]',
        "Run seeds the retry counter", "", "Run seeds the retry counter")

    validator_own_fixture_case(
        "V9  a DOC comment inside a function body is dropped, not top-level",
        '[{"file":"svc.go","line":4,"template_type":"DOC","text":"// x holds the seed '
        'value."}]',
        "", "top-level func/type/const/var/package declaration")

    custom_case("V9b a DOC comment on an unexported top-level declaration is dropped",
                v9b_body)

    validator_own_fixture_case(
        "V10  a multi-sentence DOC comment is dropped",
        '[{"file":"svc.go","line":3,"template_type":"DOC","text":"// Run seeds the '
        'counter. It also starts the loop."}]',
        "", "exactly one sentence")

    custom_case("V11 two adjacent FIELD proposals both splice -- FIELD is exempt from "
                "the stacking rule", v11_body)
    custom_case("V12 a Setup banner in a _test.go file splices", v12_body)
    custom_case("V3  a successful splice calls the shared auto_format.run_formatter on "
                "the touched file", v3_body)


def check_e2e() -> None:
    emit("\nend-to-end comment pipeline\n\n")
    first = allocate()
    second = allocate()
    third = allocate()
    submit(e2e_body, first, second, third)
    custom_case("K17 a new untracked file with a RET_ARITY_3 site is reported by a "
                "default (changed-lines) check", untracked_body)


def check_comments_hook() -> None:
    emit("\ncomments hook subcommand\n\n")
    index = allocate()
    submit(comments_hook_body, index,
           "H1  a Go file with a MISSING site produces additionalContext",
           "fixture-hook", DISCRIMINANT_FIXTURE, True)
    index = allocate()
    submit(comments_hook_body, index, "H2  a clean file produces no output",
           "fixture-hook-clean",
           'package main\n\nfunc Load(p string) (string, error) {\n\treturn "", nil\n}\n',
           False)
    custom_case("H3  a malformed payload exits 0 with no output", hook_malformed_body)
    custom_case("H4  a symlink resolving outside repo root produces no output",
                hook_symlink_body)


def build_inject_fixtures() -> str:
    root = os.path.join(WORKDIR[0], "inject")
    for name in ("empty", "full", "nested/docs", "junk", "gated"):
        os.makedirs(os.path.join(root, name))

    write_text(os.path.join(root, "full", "BACKLOG.md"), "\n".join([
        "# Project Backlog",
        "## [EPIC-01] Now, V1",
        "### [TG-01.1] Cross-Cutting",
        "#### [TSK-01.1.1] Idempotent POST /orders [P: C] [IN_PROGRESS]",
        "* **Done when:** `go test ./orders/...`",
        "#### [TSK-01.1.2] POST /orders/:id/refund [P: H] [TODO]",
        "* **Done when:** `go test ./refund/...`",
        "#### [TSK-01.1.3] Refund idempotency [P: H] [BLOCKED]",
        "* **Blocked By:** `external`",
        "  * **Reason:** the SDK exposes no idempotency key at the pinned version.",
        "* **Done when:** `go test ./refund/...`",
        "#### [TSK-01.1.4] Fix flaky test [P: L] [DONE]",
        "* **Done when:** `go test ./flaky/...`",
        "",
    ]))

    write_text(os.path.join(root, "full", "CHANGELOG.md"), "\n".join([
        "# Changelog",
        "## [Unreleased]",
        "## [0.4.2] — 2026-08-20",
        "### Added",
        "- Something.",
        "",
    ]))

    write_text(os.path.join(root, "nested", "docs", "BACKLOG.md"), "\n".join([
        "# Project Backlog",
        "#### [TSK-01.1.1] a nested story [P: M] [TODO]",
        "* **Done when:** `true`",
        "",
    ]))

    write_text(os.path.join(root, "junk", "BACKLOG.md"), "not a backlog at all\n")

    write_text(os.path.join(root, "gated", "BACKLOG.md"), "\n".join([
        "# Project Backlog",
        "#### [TSK-01.1.1] a gated story [P: M] [TODO]",
        "* **Done when:** `true`",
        "",
    ]))
    write_text(os.path.join(root, "gated", "Makefile"), "verify:\n\tbash scripts/verify.sh\n")
    return root


def check_context_injector() -> None:
    emit("\ncontext injector\n\n")
    root = build_inject_fixtures()
    full = os.path.join(root, "full")

    if IS_WINDOWS:
        skip_case("I1  reports the WIP story",
                  'ci.main() returns empty stdout for the "full" fixture on the Windows '
                  "Git Bash runner")
        skip_case("I2  counts blocked stories",
                  'same empty-stdout failure as I1 - only the "full" fixture is affected')
        skip_case("I3  reports the released version",
                  'same empty-stdout failure as I1 - only the "full" fixture is affected')
    else:
        inject_case("I1  reports the WIP story", full, "Idempotent POST /orders")
        inject_case("I2  counts blocked stories", full, "3 open, 1 BLOCKED")
        inject_case("I3  reports the released version", full, "Released version: 0.4.2")

    inject_case("I4  skips the Unreleased heading", full, "", "version: Unreleased")
    inject_case("I5  an indented note is not a story", full, "",
                "SDK exposes no idempotency key")

    if IS_WINDOWS:
        skip_case("I6  a missing verify gate is a loud warning",
                  'same empty-stdout failure as I1 - only the "full" fixture is affected')
    else:
        inject_case(
            "I6  a missing verify gate is a loud warning", full,
            "WARNING: no verify gate declared -- this repo's checks, and "
            "`comments.py check` with them, never run automatically.")

    inject_case("I15 a declared verify gate is reported plainly, unchanged",
                os.path.join(root, "gated"), "Verify gate: make verify.", "WARNING")
    inject_case("I7  silent when no backlog exists", os.path.join(root, "empty"), "",
                "Work state")
    inject_case("I8  finds a backlog under docs/", os.path.join(root, "nested"),
                "docs/BACKLOG.md")
    inject_case("I9  a backlog with no stories is fine", os.path.join(root, "junk"),
                "Nothing marked [IN_PROGRESS]")
    inject_case("I10 no version line without a changelog", os.path.join(root, "nested"),
                "", "Released version")
    inject_case("I11 outside a repo it stays silent", "", "", "Work state")
    inject_case("I12 a missing root does not crash", "/nonexistent/repo", "", "Work state")
    inject_case("I13 counts a non-zero open tally for a heading-format backlog", full,
                "Backlog: 3 open")
    inject_case("I14 a DONE story is excluded from the open tally", full, "Backlog: 3 open",
                "Fix flaky test")


def vgate_field(out: str, field: str, default: str) -> str:
    if not out:
        return default
    try:
        return json.loads(out).get(field, default)
    except (ValueError, TypeError):
        return "malformed" if field == "decision" else ""


def vgate_run(fixture: str, env: dict = None) -> str:
    return run_hook(VERIFY_GATE, '{"cwd":"%s"}' % fixture, env=env)


def presence(path: str) -> str:
    return "present" if os.path.exists(path) else "absent"


def band_marker_path(fixture: str) -> str:
    result = run_process(["git", "rev-parse", "--git-common-dir"], cwd=fixture)
    common = result.stdout.strip()
    if not os.path.isabs(common):
        common = os.path.join(fixture, common)
    digest = hashlib.sha256(os.path.realpath(common).encode("utf-8")).hexdigest()[:16]
    return os.path.join(tempfile.gettempdir(), "komodo-verify-gate-band-" + digest)


def remove_path(path: str) -> None:
    if os.path.isdir(path) and not os.path.islink(path):
        shutil.rmtree(path, ignore_errors=True)
    elif os.path.exists(path) or os.path.islink(path):
        os.remove(path)


def shim_bin(directory: str, body: str) -> str:
    os.makedirs(directory)
    write_script(os.path.join(directory, "git"), body)
    return directory


def check_verify_gate() -> None:
    emit("\nverify gate\n\n")

    if IS_WINDOWS or not shutil.which("make"):
        for label in VGATE_SKIP_LABELS:
            skip_case(label, MAKE_SKIP_REASON)
        return

    work = WORKDIR[0]
    fixture = os.path.join(work, "fixture-vgate")
    seeded_repo(fixture)
    write_text(os.path.join(fixture, "file.txt"), "dirty\n")

    makefile = os.path.join(fixture, "Makefile")
    write_text(makefile, "verify:\n\t@echo $$$$; exit 1\n")
    out = ""
    for _ in range(5):
        out = vgate_run(fixture)
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = "VG1 blocks below the warning threshold carry no streak warning"
    if decision == "block" and "consecutive" not in reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % decision, reason)

    out = vgate_run(fixture)
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = "VG2 the streak warning appears once the block count reaches the threshold"
    if decision == "block" and "consecutive" in reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % decision, reason)

    write_text(makefile, "verify:\n\t@exit 0\n")
    pass_decision = vgate_field(vgate_run(fixture), "decision", "allow")
    write_text(makefile, "verify:\n\t@exit 1\n")
    out = vgate_run(fixture)
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = "VG3 a passing verify clears the streak"
    if pass_decision == "allow" and decision == "block" and "consecutive" not in reason:
        report(index, label, "")
    else:
        report(index, label, "pass=%s next=%s" % (pass_decision, decision), reason)

    timeout_fixture = os.path.join(work, "fixture-vgate-timeout")
    os.makedirs(os.path.join(timeout_fixture, ".claude"))
    make_repo(timeout_fixture)
    write_script(os.path.join(timeout_fixture, ".claude", "verify.sh"),
                 "#!/bin/sh\nsleep 3\nexit 1\n")
    git_quiet(["add", "-A"], cwd=timeout_fixture)
    git_quiet(["commit", "-q", "-m", "init"], cwd=timeout_fixture)
    write_text(os.path.join(timeout_fixture, "file.txt"), "dirty\n")
    out = vgate_run(timeout_fixture, env_with(KOMODO_VERIFY_TIMEOUT="1"))
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = "VG4 a verify command past KOMODO_VERIFY_TIMEOUT blocks naming the limit"
    if decision == "block" and "exceeded 1 s" in reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % decision, reason)

    records = os.path.join(work, "fixture-vgate-records-only")
    os.makedirs(os.path.join(records, ".claude"))
    make_repo(records)
    marker = os.path.join(records, "marker")
    write_script(os.path.join(records, ".claude", "verify.sh"),
                 '#!/bin/sh\ntouch "%s/marker"\nexit 1\n' % records)
    git_quiet(["add", "-A"], cwd=records)
    git_quiet(["commit", "-q", "-m", "init"], cwd=records)

    for name in ("BACKLOG.md", "CHANGELOG.md"):
        write_text(os.path.join(records, name), "")
    decision = vgate_field(vgate_run(records), "decision", "allow")
    index = allocate()
    label = "VG5 records-only dirty paths skip the gate without running it"
    if decision != "block" and not os.path.exists(marker):
        report(index, label, "")
    else:
        report(index, label, "decision=%s marker=%s" % (decision, presence(marker)))

    remove_path(os.path.join(records, "BACKLOG.md"))
    if os.path.exists(marker):
        os.remove(marker)
    write_text(os.path.join(records, "x.go"), "package main\n")
    decision = vgate_field(vgate_run(records), "decision", "allow")
    index = allocate()
    label = "VG6 a non-records dirty path still runs the gate"
    if decision == "block" and os.path.exists(marker):
        report(index, label, "")
    else:
        report(index, label, "decision=%s marker=%s" % (decision, presence(marker)))

    identical = os.path.join(work, "fixture-vgate-identical")
    seeded_repo(identical)
    write_text(os.path.join(identical, "Makefile"), "verify:\n\t@exit 1\n")
    write_text(os.path.join(identical, "file.txt"), "dirty\n")

    first = vgate_field(vgate_run(identical), "decision", "allow")
    index = allocate()
    label = "VG7 the first of three identical failures blocks"
    if first == "block":
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % first)

    out = vgate_run(identical)
    second = vgate_field(out, "decision", "allow")
    second_reason = vgate_field(out, "reason", "")
    index = allocate()
    label = "VG8 the second identical failure names the repeat"
    if second == "block" and "Same failure" in second_reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % second, second_reason)

    out = vgate_run(identical)
    third = vgate_field(out, "decision", "allow")
    third_message = vgate_field(out, "systemMessage", "")
    index = allocate()
    label = "VG9 the third identical failure returns instead of blocking again"
    if third != "block" and "identical failure three times" in third_message:
        report(index, label, "")
    else:
        report(index, label,
               "decision=%s systemMessage=%s" % (third, third_message))

    real_git = shutil.which("git")
    probe = os.path.join(work, "fixture-vgate-probe-timeout")
    seeded_repo(probe)
    write_text(os.path.join(probe, "Makefile"), "verify:\n\t@exit 1\n")
    write_text(os.path.join(probe, "file.txt"), "dirty\n")
    probe_bin = shim_bin(
        os.path.join(work, "fixture-vgate-probe-timeout-bin"),
        '#!/bin/sh\nif [ "$1" = "status" ]; then\n  sleep 6\nfi\nexec "%s" "$@"\n'
        % real_git,
    )
    out = vgate_run(probe, env_with(PATH=probe_bin + os.pathsep + os.environ["PATH"]))
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = ("VG10 a slow git-status probe falls through to running verify instead of "
             "skipping")
    if decision == "block" and "is failing" in reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % decision, reason)

    band = os.path.join(work, "fixture-vgate-band")
    os.makedirs(os.path.join(band, ".claude"))
    make_repo(band)
    ran = os.path.join(band, "ran")
    write_script(os.path.join(band, ".claude", "verify.sh"),
                 '#!/bin/sh\ntouch "%s"\necho $$\nexit 1\n' % ran)
    git_quiet(["add", "-A"], cwd=band)
    git_quiet(["commit", "-q", "-m", "init"], cwd=band)
    write_text(os.path.join(band, "x.go"), "package main\n")
    band_marker = band_marker_path(band)

    def band_case(label: str, expectation: str, env: dict = None) -> None:
        if os.path.exists(ran):
            os.remove(ran)
        out = vgate_run(band, env)
        decision = vgate_field(out, "decision", "allow")
        message = vgate_field(out, "systemMessage", "")
        state = presence(ran)
        index = allocate()
        if expectation == "defer":
            good = decision != "block" and state == "absent" and "band" in message
        else:
            good = decision == "block" and state == "present"
        if good:
            report(index, label, "")
        else:
            report(index, label, "decision=%s verify=%s" % (decision, state), message)

    remove_path(band_marker)
    write_text(band_marker, "%d\n" % (int(time.time()) + 600))
    band_case("VG11 a well-formed unexpired band marker defers the full gate", "defer")

    write_text(band_marker, "soon\n")
    band_case("VG12 a malformed band marker still runs the gate", "run")

    write_text(band_marker, "%d\n" % (int(time.time()) - 60))
    band_case("VG13 an expired band marker still runs the gate", "run")

    write_text(band_marker, "%d\n" % (int(time.time()) + 86400))
    band_case("VG14 a band marker past the deferral cap still runs the gate", "run")

    remove_path(band_marker)
    os.makedirs(band_marker)
    band_case("VG15 a band marker that is not a readable file still runs the gate", "run")
    remove_path(band_marker)

    nocommon_bin = shim_bin(
        os.path.join(work, "fixture-vgate-nocommon-bin"),
        '#!/bin/sh\nif [ "$1" = "rev-parse" ] && [ "$2" = "--git-common-dir" ]; then\n'
        '  exit 1\nfi\nexec "%s" "$@"\n' % real_git,
    )
    write_text(band_marker, "%d\n" % (int(time.time()) + 600))
    if os.path.exists(ran):
        os.remove(ran)
    out = vgate_run(band, env_with(PATH=nocommon_bin + os.pathsep + os.environ["PATH"]))
    decision = vgate_field(out, "decision", "allow")
    state = presence(ran)
    index = allocate()
    label = "VG16 an unresolvable git common dir still runs the gate"
    if decision == "block" and state == "present":
        report(index, label, "")
    else:
        report(index, label, "decision=%s verify=%s" % (decision, state))
    remove_path(band_marker)


def check_verify_gate_discovery() -> None:
    emit("\nverify gate discovery\n\n")

    work = WORKDIR[0]
    both = os.path.join(work, "fixture-vgate-python-entry")
    os.makedirs(os.path.join(both, "scripts"))
    make_repo(both)
    marker = os.path.join(both, "ran-python")
    write_text(
        os.path.join(both, "scripts", "verify.py"),
        "import sys\nopen(%r, 'w').close()\nsys.exit(1)\n" % marker,
    )
    write_text(os.path.join(both, "Makefile"), "verify:\n\t@exit 0\n")
    git_quiet(["add", "-A"], cwd=both)
    git_quiet(["commit", "-q", "-m", "init"], cwd=both)
    write_text(os.path.join(both, "x.go"), "package main\n")

    out = vgate_run(both)
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    label = ("VG17 scripts/verify.py wins over a Makefile verify target and runs with no "
             "executable bit")
    if decision == "block" and os.path.exists(marker) and "scripts/verify.py" in reason:
        report(index, label, "")
    else:
        report(index, label,
               "decision=%s marker=%s" % (decision, presence(marker)), reason)

    label = "VG18 a repo with only a Makefile still resolves to make verify"
    if IS_WINDOWS or not shutil.which("make"):
        skip_case(label, MAKE_SKIP_REASON)
        return

    only_make = os.path.join(work, "fixture-vgate-make-only")
    seeded_repo(only_make)
    write_text(os.path.join(only_make, "Makefile"), "verify:\n\t@exit 1\n")
    write_text(os.path.join(only_make, "file.txt"), "dirty\n")
    out = vgate_run(only_make)
    decision = vgate_field(out, "decision", "allow")
    reason = vgate_field(out, "reason", "")
    index = allocate()
    if decision == "block" and "`make verify` is failing" in reason:
        report(index, label, "")
    else:
        report(index, label, "decision=%s" % decision, reason)


VGATE_SKIP_LABELS = [
    "VG1 blocks below the warning threshold carry no streak warning",
    "VG2 the streak warning appears once the block count reaches the threshold",
    "VG3 a passing verify clears the streak",
    "VG4 a verify command past KOMODO_VERIFY_TIMEOUT blocks naming the limit",
    "VG5 records-only dirty paths skip the gate without running it",
    "VG6 a non-records dirty path still runs the gate",
    "VG7 the first of three identical failures blocks",
    "VG8 the second identical failure names the repeat",
    "VG9 the third identical failure returns instead of blocking again",
    "VG10 a slow git-status probe falls through to running verify instead of skipping",
    "VG11 a well-formed unexpired band marker defers the full gate",
    "VG12 a malformed band marker still runs the gate",
    "VG13 an expired band marker still runs the gate",
    "VG14 a band marker past the deferral cap still runs the gate",
    "VG15 a band marker that is not a readable file still runs the gate",
    "VG16 an unresolvable git common dir still runs the gate",
]


def build_fixtures() -> None:
    work = WORKDIR[0]
    FIXTURES["main"] = os.path.join(work, "fixture-main")
    FIXTURES["feat"] = os.path.join(work, "fixture-feat")
    FIXTURES["check"] = os.path.join(work, "fixture-check")
    FIXTURES["validator"] = os.path.join(work, "fixture-validator")
    FIXTURES["v9b"] = os.path.join(work, "fixture-validator-v9b")
    FIXTURES["v11"] = os.path.join(work, "fixture-validator-v11")
    FIXTURES["v12"] = os.path.join(work, "fixture-validator-v12")
    FIXTURES["somedir"] = os.path.join(work, "somedir")
    FIXTURES["outside"] = os.path.join(work, "outside-repo")
    FIXTURES["outside2"] = os.path.join(work, "outside-repo-2")
    FIXTURES["comments_copy"] = os.path.join(work, "cw")
    FIXTURES["comments_alias"] = os.path.join(work, "Comments.PY")

    seeded_repo(FIXTURES["main"])
    os.makedirs(os.path.join(FIXTURES["main"], "claude-code"))
    seeded_repo(FIXTURES["feat"])
    git_quiet(["checkout", "-q", "-b", "feat/test-branch"], cwd=FIXTURES["feat"])

    for key in ("check", "validator", "v9b", "v11", "v12", "somedir", "outside",
                "outside2"):
        os.makedirs(FIXTURES[key])

    write_text(os.path.join(FIXTURES["validator"], "svc.go"), GO_FIXTURE)
    write_text(os.path.join(FIXTURES["v9b"], "svc.go"),
               "package main\n\nfunc helper() {}\n")
    write_text(os.path.join(FIXTURES["v11"], "svc.go"),
               "package main\n\ntype Config struct {\n\tTimeout time.Duration\n"
               "\tAPIKey  string\n}\n")
    write_text(os.path.join(FIXTURES["v12"], "svc_test.go"),
               "package main\n\nfunc TestFoo(t *testing.T) {}\n")

    shutil.copyfile(COMMENTS, FIXTURES["comments_copy"])
    os.symlink(COMMENTS, FIXTURES["comments_alias"])


def cleanup() -> None:
    shutil.rmtree(WORKDIR[0], ignore_errors=True)
    temp = tempfile.gettempdir()
    patterns = [
        os.path.join(temp, "claude-comment-grant-%s*" % SESSION),
        os.path.join(temp, "claude-comment-ledger-%s*" % SESSION),
        os.path.join(temp, "komodo-verify-gate-streak-*"),
    ]
    for pattern in patterns:
        for path in glob.glob(pattern):
            try:
                remove_path(path)
            except OSError:
                pass


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")

    WORKDIR[0] = tempfile.mkdtemp()
    workers = int(os.environ.get("TEST_HOOKS_PARALLEL") or DEFAULT_PARALLEL)
    executor = ThreadPoolExecutor(max_workers=workers)
    POOL["executor"] = executor
    try:
        build_fixtures()

        emit("\nsmoke test\n\n")
        smoke_case("S1  echo hello is allowed", "allow", "echo hello")
        smoke_case("S2  git status is allowed", "allow", "git status")
        smoke_case("S3  ls is allowed", "allow", "ls")
        smoke_case("S4  git push to a protected ref is denied", "deny",
                   "git push origin master")
        smoke_case("S5  tee into a guarded path is denied", "deny", "tee BACKLOG.md")
        smoke_case("S6  sed --in-place rewriting a file in place is denied", "deny",
                   "sed --in-place -e s/a/b/ file")
        smoke_case("S7  a redirect into a guarded path is denied", "deny",
                   "echo bad > BACKLOG.md")

        check_comments_check()
        check_git_guard()
        check_auto_format()
        check_comments_apply()
        check_e2e()
        check_comments_hook()
        check_context_injector()
        check_verify_gate()
        check_verify_gate_discovery()

        executor.shutdown(wait=True)

        passed = 0
        failed = 0
        for index in sorted(RESULTS, key=lambda n: "%d.out" % n):
            text = RESULTS[index]
            emit(text)
            if text.startswith("  PASS  "):
                passed += 1
            elif text.startswith("  FAIL  "):
                failed += 1
        emit("\n  %d passed, %d failed\n\n" % (passed, failed))
        return 1 if failed else 0
    finally:
        executor.shutdown(wait=True)
        cleanup()


if __name__ == "__main__":
    sys.exit(main())
