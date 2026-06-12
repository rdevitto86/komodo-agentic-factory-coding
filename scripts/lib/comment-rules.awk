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

function is_func_decl(t) {
  if (ENVIRON["FAMILY"] == "cfamily") {
    if (t ~ /^(func|fn)[ \t(]/) return 1
    if (t ~ /^(pub|public|private|protected|static|export|default|async)[ \t].*(fn|function)[ \t(]/) return 1
    if (t ~ /^(async[ \t]+)?function[ \t(]/) return 1
    if (t ~ /^(export[ \t]+)?(const|let|var)[ \t]+[A-Za-z_$][A-Za-z0-9_$]*[ \t]*=.*(=>|function)/) return 1
    if (t ~ /\(/ && t ~ /\)[ \t]*\{[ \t]*$/ && t ~ /^[A-Za-z_$]/ && t !~ /^(if|for|while|switch|catch|return|new|throw|else|do)[ \t(]/) return 1
    return 0
  }
  if (t ~ /^(async[ \t]+)?def[ \t]/) return 1
  if (t ~ /^[A-Za-z_][A-Za-z0-9_]*[ \t]*\(\)[ \t]*\{?[ \t]*$/) return 1
  return 0
}

function is_decl(t) {
  if (ENVIRON["FAMILY"] == "cfamily")
    return (t ~ /^(var|const|let|type|struct|class|interface|enum|trait|impl|package|import|module|use|namespace)[ \t({]/)
  return (t ~ /^(class|import|from|require)[ \t]/ || t ~ /^[A-Za-z_][A-Za-z0-9_]*[ \t]*=[^=]/)
}

function has_filler(    i) {
  for (i = 1; i <= run; i++)
    if (buf[i] ~ /[Ii]s an? (function|method|helper|class|struct|type|interface|enum|wrapper|utility|component|constructor|handler)([ .,]|$)/) return 1
  return 0
}

function sentence_count(    i, text, n) {
  text = ""
  for (i = 1; i <= run; i++) {
    if (is_delim_only(trim(buf[i]))) continue
    text = text " " decomment(buf[i])
  }
  n = gsub(/[.!?]+["')\]]*([ \t]|$)/, "", text)
  return n
}

function reset_run() { run = 0; cl = 0; run_added = 0; pending_blank = 0 }

function evaluate_run(t) {
  if (is_func_decl(t)) {
    if (sentence_count() > 2 || cl > 6) report("function doc exceeds 2 sentences")
    else if (has_filler()) report("name-restating filler doc")
  } else if (is_decl(t)) {
    report("comment on a declaration")
  } else if (cl > 1) {
    report("multi-line prose comment block")
  }
}

function report(reason,    i) {
  if (ENVIRON["MARKED"] == "1" && !run_added) return
  for (i = 1; i <= run; i++) print "    [" reason "] " buf[i]
}

function report_line(reason, line, added) {
  if (ENVIRON["MARKED"] == "1" && !added) return
  print "    [" reason "] " line
}

function is_control_flow(t) {
  if (ENVIRON["FAMILY"] == "cfamily")
    return (t ~ /^(if|for|while|switch|case|default|else|do|try|catch|finally|select)([ \t({:]|$)/ || t ~ /^\}[ \t]*(else|while|catch|finally)\b/)
  return (t ~ /^(if|elif|else|for|while|try|except|finally|with|case)([ \t:]|$)/)
}

function decomment(s) {
  sub(/^[ \t]*\/\*+[ \t]*/, "", s)
  sub(/\*+\/[ \t]*$/, "", s)
  sub(/^[ \t]*\*+[ \t]*/, "", s)
  sub(/^[ \t]*\/\/[ \t]*/, "", s)
  sub(/^[ \t]*#[ \t]*/, "", s)
  return trim(s)
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
  if (t == "") {
    if (cl > 1) {
      report("detached multi-line comment block")
      reset_run()
    } else if (cl == 1 && !pending_blank) {
      pending_blank = 1
    } else {
      reset_run()
    }
    next
  }
  if (is_exempt(t)) next
  if (ENVIRON["FAMILY"] == "hash" && t ~ /^@/) next
  if (is_comment(t)) {
    if (pending_blank) reset_run()
    run++
    buf[run] = line
    if (!is_delim_only(t)) cl++
    if (mark == "A" || ENVIRON["MARKED"] != "1") run_added = 1
    next
  }
  if (run > 0) {
    evaluate_run(t)
    reset_run()
  }
  trail = trailing_comment(t)
  if (trail != "" && !is_func_decl(t) && !is_control_flow(t))
    report_line("trailing comment on declaration", line, (mark == "A" || ENVIRON["MARKED"] != "1"))
}
