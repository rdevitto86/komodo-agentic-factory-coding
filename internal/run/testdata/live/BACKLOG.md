# Backlog

### [TG-01.1] Greet by name
```yaml
type: feat
version: 0.1.0
mode: single
```
* **Why:** the smallest two-task group that builds, reviews, repairs and ships.

#### [TSK-01.1.1] Hello returns a greeting for a name [P: M] [READY]
```yaml
files: [greet/greet.go]
done_when:
  - go build ./greet/...
context:
  - "package greet; func Hello(name string) string returns \"Hello, \" + name + \"!\", with a godoc line"
```

#### [TSK-01.1.2] A table test covers Hello [P: M] [READY]
```yaml
files: [greet/greet_test.go]
done_when:
  - go test ./greet/...
depends_on: [TSK-01.1.1]
context:
  - "one table-driven test with two names"
```
