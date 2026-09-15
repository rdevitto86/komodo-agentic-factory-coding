.PHONY: verify test validate comments

verify:
	python3 scripts/verify.py

test:
	python3 scripts/test_hooks.py

validate:
	python3 scripts/validate.py

comments:
	python3 claude-code/hooks/comments.py check
