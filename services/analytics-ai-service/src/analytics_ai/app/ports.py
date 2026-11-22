"""Portas do serviço. Todo método recebe o tenant: o isolamento não depende do chamador lembrar."""

from __future__ import annotations

from contextlib import AbstractAsyncContextManager
from typing import Protocol
from uuid import UUID

from analytics_ai.domain.models import (
    AssessmentFact,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    TenantSettings,
)


class ReadModelRepository(Protocol):
    async def claim_event(self, tenant_id: UUID, event_id: UUID) -> bool:
        """Registra o evento como processado. False se já tinha sido (deduplicação)."""

    async def save_tenant_settings(self, settings: TenantSettings) -> None: ...
    async def get_tenant_settings(self, tenant_id: UUID) -> TenantSettings | None: ...
    async def upsert_employment(self, record: EmploymentRecord) -> None: ...
    async def get_employment(self, tenant_id: UUID, employee_id: UUID) -> EmploymentRecord | None: ...
    async def list_employment(self, tenant_id: UUID) -> list[EmploymentRecord]: ...
    async def save_assessment(self, fact: AssessmentFact) -> None: ...
    async def latest_assessment(self, tenant_id: UUID, employee_id: UUID) -> AssessmentFact | None: ...
    async def save_nine_box(self, fact: NineBoxFact) -> None: ...
    async def list_nine_box(self, tenant_id: UUID, cycle_id: UUID) -> list[NineBoxFact]: ...
    async def save_survey(self, fact: SurveyFact) -> None: ...
    async def get_survey(self, tenant_id: UUID, survey_id: UUID) -> SurveyFact | None: ...


class UnitOfWork(Protocol):
    """Unidade de trabalho de um tenant: tudo que acontece dentro vale junto ou não vale.

    O consumidor de eventos precisa disso: registrar o evento como processado e gravar o efeito na mesma transação.
    Se o efeito falha, o registro some junto, e a reentrega do Kafka aplica o evento em vez de descartá-lo como duplicado.
    """

    def unit_of_work(self, tenant_id: UUID) -> AbstractAsyncContextManager[ReadModelRepository]: ...


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
