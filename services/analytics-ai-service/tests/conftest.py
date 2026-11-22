from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import date
from pathlib import Path
from uuid import UUID

import pytest

from analytics_ai.adapters.http.api import Deps
from analytics_ai.adapters.http.auth import InvalidTokenError, Principal
from analytics_ai.adapters.memory import InMemoryRepository
from analytics_ai.app.ai.budget import InMemoryAiBudget
from analytics_ai.app.ai.runner import AiRunner
from analytics_ai.app.ai.service import AiService
from analytics_ai.app.analytics_service import AnalyticsService
from analytics_ai.domain.models import EmploymentRecord

FIXTURES = Path(__file__).parent / "fixtures"
TENANT_A = UUID("11111111-1111-4111-8111-111111111111")
TENANT_B = UUID("99999999-9999-4999-8999-999999999999")


def fixture(name: str) -> dict:
    return json.loads((FIXTURES / f"{name}.json").read_text())


@dataclass
class FakeResult:
    text: str
    total_tokens: int = 100


class FakeLLM:
    """Devolve respostas pré-definidas, em ordem, e guarda os prompts recebidos."""

    def __init__(self, *replies: str) -> None:
        self.replies = list(replies)
        self.calls: list[dict[str, str]] = []

    async def complete(self, *, system: str, user: str, max_tokens: int) -> FakeResult:
        self.calls.append({"system": system, "user": user, "max_tokens": str(max_tokens)})
        return FakeResult(self.replies.pop(0))


class TokenTable:
    """Verificador de teste: o token é a chave de um Principal."""

    def __init__(self) -> None:
        self.table: dict[str, Principal] = {}

    def verify(self, token: str) -> Principal:
        if token not in self.table:
            raise InvalidTokenError("desconhecido")
        return self.table[token]


@pytest.fixture
def repo() -> InMemoryRepository:
    return InMemoryRepository()


@pytest.fixture
def tokens() -> TokenTable:
    t = TokenTable()
    t.table["manager-a"] = Principal("u1", TENANT_A, frozenset({"manager"}))
    t.table["manager-b"] = Principal("u2", TENANT_B, frozenset({"manager"}))
    t.table["employee-a"] = Principal("u3", TENANT_A, frozenset({"employee"}))
    return t


def make_deps(
    repo: InMemoryRepository, tokens: TokenTable, llm: FakeLLM, budget: InMemoryAiBudget | None = None
) -> Deps:
    budget = budget or InMemoryAiBudget()
    return Deps(
        verifier=tokens,
        analytics=AnalyticsService(repo, 3),
        ai=AiService(AiRunner(llm, budget), repo),
        model_name="fake-model",
        min_group_size=3,
    )


def people(
    tenant: UUID, area: str, n: int, *, terminated: int = 0, hired: str = "2024-01-10"
) -> list[EmploymentRecord]:
    return [
        EmploymentRecord(
            tenant_id=tenant,
            employee_id=UUID(int=hash((tenant, area, i)) & ((1 << 128) - 1)),
            hired_on=date.fromisoformat(hired),
            area=area,
            unit="Matriz",
            birth_year=1990,
            terminated_on=date(2026, 8, 1) if i < terminated else None,
        )
        for i in range(n)
    ]
