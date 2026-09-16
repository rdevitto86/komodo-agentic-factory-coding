.PHONY: verify test build

verify:
	npm run lint
	npm run typecheck
	npm test
	npm run build
	python3 -m komodo comments check

test:
	npm test

build:
	npm run build
