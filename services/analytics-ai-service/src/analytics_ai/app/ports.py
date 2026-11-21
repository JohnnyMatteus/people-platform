"""Portas do serviço. Todo método recebe o tenant: o isolamento não depende do chamador lembrar."""

from __future__ import annotations

from typing import Protocol
from uuid import UUID

from analytics_ai.domain.models import AssessmentFact, EmploymentRecord, NineBoxFact, SurveyFact


class ReadModelRepository(Protocol):
    async def claim_event(self, tenant_id: UUID, event_id: UUID) -> bool:
        """Registra o evento como processado. False se já tinha sido (deduplicação)."""

    async def upsert_employment(self, record: EmploymentRecord) -> None: ...
    async def list_employment(self, tenant_id: UUID) -> list[EmploymentRecord]: ...
    async def save_assessment(self, fact: AssessmentFact) -> None: ...
    async def latest_assessment(self, tenant_id: UUID, employee_id: UUID) -> AssessmentFact | None: ...
    async def save_nine_box(self, fact: NineBoxFact) -> None: ...
    async def list_nine_box(self, tenant_id: UUID, cycle_id: UUID) -> list[NineBoxFact]: ...
    async def save_survey(self, fact: SurveyFact) -> None: ...
    async def get_survey(self, tenant_id: UUID, survey_id: UUID) -> SurveyFact | None: ...


class LLMResult(Protocol):
    @property
    def text(self) -> str: ...
    @property
    def total_tokens(self) -> int: ...


class LLMClient(Protocol):
    async def complete(self, *, system: str, user: str, max_tokens: int) -> LLMResult: ...


class AiBudget(Protocol):
    async def reserve(self, tenant_id: UUID, resource: str) -> None:
        """Levanta BudgetExceededError se a cota de chamadas acabou."""

    async def record_tokens(self, tenant_id: UUID, tokens: int) -> None: ...
