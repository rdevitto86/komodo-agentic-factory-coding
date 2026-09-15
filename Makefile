.PHONY: verify test validate comments

verify: test validate comments

test:
	bash scripts/test-hooks.sh

validate:
	python3 scripts/validate.py

comments:
	python3 claude-code/hooks/comments.py check
