.PHONY: verify test lint doctor install hooks

verify:
	python3 scripts/verify.py
	python3 -m komodo release check

test:
	python3 -m unittest discover -s tests -q

lint:
	python3 -m komodo comments check

doctor:
	python3 -m komodo doctor

install:
	python3 -m komodo install

hooks:
	python3 -m komodo hooks install .
