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
