"""Orçamento de IA: chamadas por tenant e recurso por dia, e teto de tokens diário por tenant."""

from __future__ import annotations

from collections import defaultdict
from collections.abc import Callable
from datetime import UTC, date, datetime
from uuid import UUID

from .runner import BudgetExceededError


class InMemoryAiBudget:
    def __init__(
        self,
        *,
        daily_calls_per_resource: int = 50,
        daily_tokens: int = 200_000,
        today: Callable[[], date] = lambda: datetime.now(UTC).date(),
    ) -> None:
        self._max_calls = daily_calls_per_resource
        self._max_tokens = daily_tokens
        self._today = today
        self._calls: dict[tuple[date, UUID, str], int] = defaultdict(int)
        self._tokens: dict[tuple[date, UUID], int] = defaultdict(int)

    async def reserve(self, tenant_id: UUID, resource: str) -> None:
        day = self._today()
        if self._tokens[(day, tenant_id)] >= self._max_tokens:
            raise BudgetExceededError("tokens")
        if self._calls[(day, tenant_id, resource)] >= self._max_calls:
            raise BudgetExceededError(resource)
        self._calls[(day, tenant_id, resource)] += 1

    async def record_tokens(self, tenant_id: UUID, tokens: int) -> None:
        self._tokens[(self._today(), tenant_id)] += tokens
