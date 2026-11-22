"""Modelos dos eventos consumidos, espelhando contracts/asyncapi/schemas/*.v1.json."""

from __future__ import annotations

from datetime import date, datetime
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field


class Envelope(BaseModel):
    model_config = ConfigDict(extra="ignore")

    event_id: UUID
    event_type: str
    event_version: int = Field(ge=1)
    tenant_id: UUID
    aggregate_id: UUID
    occurred_at: datetime
    producer: str
    data: dict[str, object]


class _Data(BaseModel):
    model_config = ConfigDict(extra="forbid")


class CandidatoContratadoData(_Data):
    job_id: UUID
    application_id: UUID
    position_title: str
    unit: str
    start_date: date


class ColaboradorAtualizadoData(_Data):
    employee_id: UUID
    hired_on: date
    area: str = Field(max_length=80)
    unit: str = Field(max_length=80)
    age_band: str = Field(pattern=r"^(<25|25-34|35-44|45-54|55\+|nao-informado)$")


class ColaboradorDesligadoData(_Data):
    employee_id: UUID
    terminated_on: date
    voluntary: bool | None = None
    area: str = Field(max_length=80)
    unit: str = Field(max_length=80)
    age_band: str = Field(pattern=r"^(<25|25-34|35-44|45-54|55\+|nao-informado)$")


class TenantCriadoData(_Data):
    slug: str
    name: str
    plan: str
    features: list[str]


class GapData(BaseModel):
    competency: str
    gap: int = Field(ge=0)


class AvaliacaoConcluidaData(_Data):
    cycle_id: UUID
    employee_id: UUID
    performance_score: str
    potential_score: str
    competency_gaps: list[GapData]


class NineBoxCalibradoData(_Data):
    cycle_id: UUID
    employee_id: UUID
    quadrant: str = Field(pattern=r"^Q[1-9]$")
    calibrated: bool


class SegmentData(_Data):
    segment_key: str
    respondents: int = Field(ge=3)
    scores: dict[str, float]


class PesquisaClimaEncerradaData(_Data):
    survey_id: UUID
    min_group_size: int = Field(ge=3)
    segments: list[SegmentData]
