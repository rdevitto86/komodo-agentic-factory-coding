.PHONY: verify test validate comments

verify: test validate comments

test:
	python3 scripts/test_hooks.py

validate:
	python3 scripts/validate.py

comments:
	python3 claude-code/hooks/comments.py check
