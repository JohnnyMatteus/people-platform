"""API HTTP (FastAPI). Erros no formato application/problem+json."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Annotated
from uuid import UUID

from fastapi import FastAPI, HTTPException, Query, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from pydantic import BaseModel

from analytics_ai.app.ai.runner import AiOutputInvalidError, AiProviderError, BudgetExceededError
from analytics_ai.app.ai.schemas import (
    AiResponse,
    ClimateActionPlan,
    ClimateActionPlanRequest,
    FeedbackDraft,
    FeedbackDraftRequest,
    IdpSuggestions,
    IdpSuggestionsRequest,
    OneOnOneBrief,
    OneOnOneBriefRequest,
)
from analytics_ai.app.ai.service import AiService, NotFoundError
from analytics_ai.app.analytics_service import AnalyticsService
from analytics_ai.domain.analytics import GroupIndicators, InvalidPeriodError
from analytics_ai.domain.models import GroupBy

from .auth import CurrentPrincipal, TokenVerifier


class IndicatorsOut(BaseModel):
    group: str
    headcount_start: int
    headcount_end: int
    hires: int
    terminations: int
    turnover_rate: float
    avg_tenure_months: float


class TurnoverOut(BaseModel):
    period: str
    group_by: GroupBy
    min_group_size: int
    groups: list[IndicatorsOut]
    suppressed_groups: int
    total: IndicatorsOut | None


class QuadrantOut(BaseModel):
    quadrant: str
    count: int


class NineBoxOut(BaseModel):
    cycle_id: UUID
    min_group_size: int
    quadrants: list[QuadrantOut]
    suppressed_quadrants: int


@dataclass(frozen=True, slots=True)
class Deps:
    verifier: TokenVerifier
    analytics: AnalyticsService
    ai: AiService
    model_name: str
    min_group_size: int


def _problem(status: int, code: str, title: str) -> JSONResponse:
    return JSONResponse(
        {"type": f"about:blank#{code}", "title": title, "status": status, "code": code},
        status_code=status,
        media_type="application/problem+json",
    )


def _ind(g: GroupIndicators) -> IndicatorsOut:
    return IndicatorsOut(**{f: getattr(g, f) for f in IndicatorsOut.model_fields})


def create_app(deps: Deps) -> FastAPI:
    app = FastAPI(title="People Platform - analytics-ai-service", version="0.1.0")
    app.state.verifier = deps.verifier

    @app.exception_handler(HTTPException)
    async def _http(_: Request, exc: HTTPException) -> JSONResponse:
        return _problem(exc.status_code, str(exc.detail), str(exc.detail))

    @app.exception_handler(RequestValidationError)
    async def _validation(_: Request, __: RequestValidationError) -> JSONResponse:
        return _problem(422, "validation_error", "Requisição inválida")

    @app.exception_handler(AiOutputInvalidError)
    async def _invalid(_: Request, __: AiOutputInvalidError) -> JSONResponse:
        return _problem(422, "ai_output_invalid", "Saída da IA inválida após nova tentativa")

    @app.exception_handler(BudgetExceededError)
    async def _budget(_: Request, __: BudgetExceededError) -> JSONResponse:
        return _problem(429, "ai_budget_exceeded", "Cota de IA esgotada")

    @app.exception_handler(AiProviderError)
    async def _provider(_: Request, __: AiProviderError) -> JSONResponse:
        return _problem(502, "ai_provider_unavailable", "Provedor de IA indisponível")

    @app.exception_handler(NotFoundError)
    async def _not_found(_: Request, __: NotFoundError) -> JSONResponse:
        return _problem(404, "not_found", "Dados não encontrados")

    @app.exception_handler(InvalidPeriodError)
    async def _period(_: Request, __: InvalidPeriodError) -> JSONResponse:
        return _problem(422, "invalid_period", "Período inválido")

    @app.get("/healthz")
    async def healthz() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/v1/analytics/turnover", response_model=TurnoverOut)
    async def turnover(
        principal: CurrentPrincipal, group_by: Annotated[GroupBy, Query()], period: Annotated[str, Query()]
    ) -> TurnoverOut:
        report = await deps.analytics.turnover(principal.tenant_id, group_by, period)
        return TurnoverOut(
            period=report.period,
            group_by=report.group_by,
            min_group_size=report.min_group_size,
            groups=[_ind(g) for g in report.groups],
            suppressed_groups=report.suppressed_groups,
            total=_ind(report.total) if report.total else None,
        )

    @app.get("/v1/analytics/nine-box", response_model=NineBoxOut)
    async def nine_box(principal: CurrentPrincipal, cycle_id: Annotated[UUID, Query()]) -> NineBoxOut:
        shown, suppressed = await deps.analytics.nine_box(principal.tenant_id, cycle_id)
        return NineBoxOut(
            cycle_id=cycle_id,
            min_group_size=deps.min_group_size,
            quadrants=[QuadrantOut(quadrant=q.quadrant, count=q.count) for q in shown],
            suppressed_quadrants=suppressed,
        )

    @app.post("/v1/ai/feedback-drafts", response_model=AiResponse[FeedbackDraft])
    async def feedback(principal: CurrentPrincipal, body: FeedbackDraftRequest) -> AiResponse[FeedbackDraft]:
        out, version = await deps.ai.draft_feedback(principal.tenant_id, body)
        return AiResponse(suggestion=out, model=deps.model_name, prompt_version=version)

    @app.post("/v1/ai/idp-suggestions", response_model=AiResponse[IdpSuggestions])
    async def idp(principal: CurrentPrincipal, body: IdpSuggestionsRequest) -> AiResponse[IdpSuggestions]:
        out, version = await deps.ai.suggest_idp(principal.tenant_id, body.employee_id)
        return AiResponse(suggestion=out, model=deps.model_name, prompt_version=version)

    @app.post("/v1/ai/climate-action-plans", response_model=AiResponse[ClimateActionPlan])
    async def climate(
        principal: CurrentPrincipal, body: ClimateActionPlanRequest
    ) -> AiResponse[ClimateActionPlan]:
        out, version = await deps.ai.draft_climate_plan(principal.tenant_id, body.survey_id)
        return AiResponse(suggestion=out, model=deps.model_name, prompt_version=version)

    @app.post("/v1/ai/one-on-one-briefs", response_model=AiResponse[OneOnOneBrief])
    async def one_on_one(
        principal: CurrentPrincipal, body: OneOnOneBriefRequest
    ) -> AiResponse[OneOnOneBrief]:
        out, version = await deps.ai.one_on_one_brief(principal.tenant_id, body)
        return AiResponse(suggestion=out, model=deps.model_name, prompt_version=version)

    return app
