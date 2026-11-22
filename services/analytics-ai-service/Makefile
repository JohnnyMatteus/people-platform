PY ?= python3.12

.PHONY: venv lint test
venv: ## Cria o virtualenv com as dependências de desenvolvimento
	$(PY) -m venv .venv && .venv/bin/pip install -q -e ".[dev]"

lint: ## ruff + mypy
	.venv/bin/ruff check . && .venv/bin/ruff format --check . && .venv/bin/mypy src

test: ## Testes unitários (LLM falso, sem rede)
	.venv/bin/pytest -q
