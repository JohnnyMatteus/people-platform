"""Casos de uso de people analytics. Só devolvem agregados com k mínimo."""

from __future__ import annotations

from dataclasses import replace
from datetime import date
from uuid import UUID

from analytics_ai.domain.analytics import (
    QuadrantCount,
    TurnoverReport,
    nine_box_distribution,
    turnover_report,
)
from analytics_ai.domain.models import GroupBy

from .ports import ReadModelRepository


class AnalyticsService:
    def __init__(self, repo: ReadModelRepository, min_group_size: int) -> None:
        self._repo = repo
        self._k = min_group_size

    async def turnover(self, tenant_id: UUID, group_by: GroupBy, period: str) -> TurnoverReport:
        return turnover_report(await self._repo.list_employment(tenant_id), group_by, period, self._k)

    async def nine_box(self, tenant_id: UUID, cycle_id: UUID) -> tuple[list[QuadrantCount], int]:
        return nine_box_distribution(await self._repo.list_nine_box(tenant_id, cycle_id), self._k)

    async def record_termination(self, tenant_id: UUID, employee_id: UUID, terminated_on: date) -> bool:
        """Registra o desligamento. Ainda sem evento no contrato: chamado por quem o receber."""
        current = next(
            (r for r in await self._repo.list_employment(tenant_id) if r.employee_id == employee_id), None
        )
        if current is None:
            return False
        await self._repo.upsert_employment(replace(current, terminated_on=terminated_on))
        return True
