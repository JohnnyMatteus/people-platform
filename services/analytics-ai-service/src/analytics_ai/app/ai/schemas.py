"""Schemas de saída (Pydantic) e de entrada dos recursos de IA. A saída do LLM só passa por aqui."""

from __future__ import annotations

from enum import StrEnum
from typing import Annotated, Literal
from uuid import UUID

from pydantic import BaseModel, ConfigDict, Field, StringConstraints

Short = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=300)]
Text = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=2000)]


class _Strict(BaseModel):
    model_config = ConfigDict(extra="forbid")


# ---------------------------------------------------------------- saídas
class FeedbackDraft(_Strict):
    draft: Annotated[str, StringConstraints(strip_whitespace=True, min_length=20, max_length=2000)]
    tone: Literal["construtivo", "reconhecimento", "direto"]
    improvement_tips: list[Short] = Field(default_factory=list, max_length=5)


class IdpAction(_Strict):
    competency: Short
    action: Text
    kind: Literal["curso", "mentoria", "projeto", "leitura", "pratica"]
    horizon_days: int = Field(ge=7, le=365)


class IdpSuggestions(_Strict):
    actions: list[IdpAction] = Field(min_length=1, max_length=6)


class ClimateAction(_Strict):
    title: Short
    dimension: Short
    owner_role: Short
    horizon_days: int = Field(ge=7, le=365)
    rationale: Text


class ClimateActionPlan(_Strict):
    summary: Text
    actions: list[ClimateAction] = Field(min_length=1, max_length=6)


class OneOnOneBrief(_Strict):
    talking_points: list[Short] = Field(min_length=1, max_length=7)
    follow_ups: list[Short] = Field(default_factory=list, max_length=5)
    praise_to_mention: list[Short] = Field(default_factory=list, max_length=5)


# ---------------------------------------------------------------- entradas
class FeedbackTone(StrEnum):
    CONSTRUCTIVE = "construtivo"
    RECOGNITION = "reconhecimento"
    DIRECT = "direto"


class FeedbackDraftRequest(_Strict):
    recipient_name: Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=80)]
    topics: list[Short] = Field(min_length=1, max_length=8)
    tone: FeedbackTone = FeedbackTone.CONSTRUCTIVE


class IdpSuggestionsRequest(_Strict):
    employee_id: UUID


class ClimateActionPlanRequest(_Strict):
    survey_id: UUID


class OneOnOneBriefRequest(_Strict):
    employee_name: Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=80)]
    previous_agenda_items: list[Short] = Field(default_factory=list, max_length=10)
    recent_feedback_summaries: list[Short] = Field(default_factory=list, max_length=10)
    recent_praises: list[Short] = Field(default_factory=list, max_length=10)


class AiResponse[T: BaseModel](_Strict):
    """Envelope das respostas de IA: sempre sugestão, nunca aplicada automaticamente."""

    suggestion: T
    requires_confirmation: Literal[True] = True
    model: str
    prompt_version: str
