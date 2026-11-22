"""Repositório em memória. Particiona tudo por tenant, como o RLS faria no Postgres."""

from __future__ import annotations

import copy
from collections import defaultdict
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from uuid import UUID

from analytics_ai.domain.models import (
    AssessmentFact,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    TenantSettings,
)


class InMemoryRepository:
    def __init__(self) -> None:
        self._settings: dict[UUID, TenantSettings] = {}
        self._events: dict[UUID, set[UUID]] = defaultdict(set)
        self._employment: dict[UUID, dict[UUID, EmploymentRecord]] = defaultdict(dict)
        self._assessments: dict[UUID, dict[UUID, AssessmentFact]] = defaultdict(dict)
        self._nine_box: dict[UUID, dict[tuple[UUID, UUID], NineBoxFact]] = defaultdict(dict)
        self._surveys: dict[UUID, dict[UUID, SurveyFact]] = defaultdict(dict)

    @asynccontextmanager
    async def unit_of_work(self, tenant_id: UUID) -> AsyncIterator[InMemoryRepository]:
        """Tudo ou nada: se o bloco falha, o estado volta ao que era (como o rollback do Postgres)."""
        snapshot = copy.deepcopy(self.__dict__)
        try:
            yield self
        except BaseException:
            self.__dict__.clear()
            self.__dict__.update(snapshot)
            raise

    async def claim_event(self, tenant_id: UUID, event_id: UUID) -> bool:
        if event_id in self._events[tenant_id]:
            return False
        self._events[tenant_id].add(event_id)
        return True

    async def save_tenant_settings(self, settings: TenantSettings) -> None:
        self._settings[settings.tenant_id] = settings

    async def get_tenant_settings(self, tenant_id: UUID) -> TenantSettings | None:
        return self._settings.get(tenant_id)

    async def upsert_employment(self, record: EmploymentRecord) -> None:
        self._employment[record.tenant_id][record.employee_id] = record

    async def get_employment(self, tenant_id: UUID, employee_id: UUID) -> EmploymentRecord | None:
        return self._employment[tenant_id].get(employee_id)

    async def list_employment(self, tenant_id: UUID) -> list[EmploymentRecord]:
        return list(self._employment[tenant_id].values())

    async def save_assessment(self, fact: AssessmentFact) -> None:
        self._assessments[fact.tenant_id][fact.employee_id] = fact

    async def latest_assessment(self, tenant_id: UUID, employee_id: UUID) -> AssessmentFact | None:
        return self._assessments[tenant_id].get(employee_id)

    async def save_nine_box(self, fact: NineBoxFact) -> None:
        self._nine_box[fact.tenant_id][(fact.cycle_id, fact.employee_id)] = fact

    async def list_nine_box(self, tenant_id: UUID, cycle_id: UUID) -> list[NineBoxFact]:
        return [f for (cycle, _), f in self._nine_box[tenant_id].items() if cycle == cycle_id]

    async def save_survey(self, fact: SurveyFact) -> None:
        self._surveys[fact.tenant_id][fact.survey_id] = fact

    async def get_survey(self, tenant_id: UUID, survey_id: UUID) -> SurveyFact | None:
        return self._surveys[tenant_id].get(survey_id)

    async def erase_subject(self, tenant_id: UUID, subject_id: UUID) -> int:
        removed = 0
        if self._employment[tenant_id].pop(subject_id, None) is not None:
            removed += 1
        if self._assessments[tenant_id].pop(subject_id, None) is not None:
            removed += 1
        for key in [k for k in self._nine_box[tenant_id] if k[1] == subject_id]:
            del self._nine_box[tenant_id][key]
            removed += 1
        return removed
