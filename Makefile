.PHONY: verify test validate comments

verify: test validate comments

test:
	bash scripts/test-hooks.sh

validate:
	bash scripts/validate.sh

comments:
	python3 claude-code/hooks/comments.py check
