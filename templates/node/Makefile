.PHONY: verify test build

verify:
	npm run lint
	npm run typecheck
	npm test
	npm run build
	python3 ~/.claude/hooks/comments.py check

test:
	npm test

build:
	npm run build
