function trim(s) { sub(/^[ \t]+/, "", s); return s }

function is_exempt(t) {
  if (t ~ /──/) return 1
  if (t ~ /^#!/) return 1
  if (t ~ /^\/\/go:/) return 1
  if (t ~ /^\/\/[[:space:]]*\+build/) return 1
  if (t ~ /^\/\/nolint/) return 1
  if (t ~ /^\/\/[[:space:]]*export/) return 1
  if (t ~ /^\/\/[[:space:]]*Code generated/) return 1
  if (t ~ /^\/[\/\*][[:space:]]*SPDX/) return 1
  if (t ~ /eslint-(disable|enable)/) return 1
  if (t ~ /prettier-ignore/) return 1
  if (t ~ /@ts-(ignore|expect-error|nocheck)/) return 1
  if (t ~ /istanbul ignore/) return 1
  if (t ~ /^#[[:space:]]*(noqa|type:|pragma|pylint:|mypy:|fmt:|isort:|ruff:|yapf:|shellcheck)/) return 1
  if (t ~ /-\*-[[:space:]]*coding/) return 1
  if (t ~ /^#[[:space:]]*SPDX/) return 1
  return 0
}

function is_comment(t) {
  if (ENVIRON["FAMILY"] == "cfamily")
    return (t ~ /^\/\// || t ~ /^\/\*/ || (t ~ /^\*/ && t !~ /^\*[a-zA-Z_(]/))
  return (t ~ /^#/)
}

function is_delim_only(t) {
  return (t == "/*" || t == "/**" || t == "*/" || t == "*")
}

function reset_run() { run = 0; run_added = 0 }

function report(reason,    i) {
  if (ENVIRON["MARKED"] == "1" && !run_added) return
  for (i = 1; i <= run; i++) print "    [" reason "] " buf[i]
}

function report_line(reason, line, added) {
  if (ENVIRON["MARKED"] == "1" && !added) return
  print "    [" reason "] " line
}

# trailing-marker scan can't tell a real comment from "//" inside a string literal.
function trailing_comment(t,    pos, marker, candidate) {
  marker = (ENVIRON["FAMILY"] == "cfamily") ? "//" : "#"
  pos = index(t, " " marker)
  if (!pos) pos = index(t, "\t" marker)
  if (!pos) return ""
  candidate = trim(substr(t, pos + 1))
  if (is_exempt(candidate)) return ""
  return candidate
}

BEGIN { reset_run() }

{
  line = $0
  mark = ""
  if (ENVIRON["MARKED"] == "1") {
    mark = substr(line, 1, 1)
    line = substr(line, 3)
    if (mark == "R") { reset_run(); next }
  }
  t = trim(line)
  if (t == "") next
  if (is_exempt(t)) next
  if (ENVIRON["FAMILY"] == "hash" && t ~ /^@/) next
  if (is_comment(t)) {
    if (!is_delim_only(t)) {
      run++
      buf[run] = line
      if (mark == "A" || ENVIRON["MARKED"] != "1") run_added = 1
    }
    next
  }
  if (run > 0) {
    report("comment not allowed")
    reset_run()
  }
  trail = trailing_comment(t)
  if (trail != "")
    report_line("trailing comment not allowed", line, (mark == "A" || ENVIRON["MARKED"] != "1"))
}

END {
  if (run > 0) report("comment not allowed")
}
