"""Modelos do domínio de analytics. Sem dependência de HTTP, Kafka ou banco."""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from enum import StrEnum
from uuid import UUID

UNKNOWN = "nao-informado"


class GroupBy(StrEnum):
    AREA = "area"
    UNIT = "unit"
    TENURE_BAND = "tenure_band"
    AGE_BAND = "age_band"
    GENDER = "gender"


@dataclass(frozen=True, slots=True)
class EmploymentRecord:
    """Um vínculo. O employee_id é um identificador opaco; nunca sai em respostas."""

    tenant_id: UUID
    employee_id: UUID
    hired_on: date
    area: str = UNKNOWN
    unit: str = UNKNOWN
    birth_year: int | None = None
    gender: str = UNKNOWN
    terminated_on: date | None = None

    def active_on(self, day: date) -> bool:
        return self.hired_on <= day and (self.terminated_on is None or self.terminated_on > day)

    def active_during(self, start: date, end: date) -> bool:
        return self.hired_on <= end and (self.terminated_on is None or self.terminated_on >= start)


@dataclass(frozen=True, slots=True)
class CompetencyGap:
    competency: str
    gap: int


@dataclass(frozen=True, slots=True)
class AssessmentFact:
    tenant_id: UUID
    employee_id: UUID
    cycle_id: UUID
    gaps: tuple[CompetencyGap, ...]


@dataclass(frozen=True, slots=True)
class NineBoxFact:
    tenant_id: UUID
    employee_id: UUID
    cycle_id: UUID
    quadrant: str


@dataclass(frozen=True, slots=True)
class SurveySegment:
    segment_key: str
    respondents: int
    scores: dict[str, float]


@dataclass(frozen=True, slots=True)
class SurveyFact:
    tenant_id: UUID
    survey_id: UUID
    min_group_size: int
    segments: tuple[SurveySegment, ...]
