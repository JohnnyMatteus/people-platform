"""Projeta eventos Kafka nos read models. Idempotente por event_id."""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable
from typing import Any

from pydantic import ValidationError

from analytics_ai.domain.models import (
    AssessmentFact,
    CompetencyGap,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    SurveySegment,
)

from .events import (
    AvaliacaoConcluidaData,
    CandidatoContratadoData,
    Envelope,
    NineBoxCalibradoData,
    PesquisaClimaEncerradaData,
)
from .ports import ReadModelRepository

log = logging.getLogger(__name__)


class InvalidEventError(ValueError):
    """Evento fora do contrato. Não adianta tentar de novo."""


class EventIngestor:
    def __init__(self, repo: ReadModelRepository) -> None:
        self._repo = repo
        self._handlers: dict[str, Callable[[Envelope], Awaitable[None]]] = {
            "CandidatoContratado": self._hired,
            "AvaliacaoConcluida": self._assessment,
            "NineBoxCalibrado": self._nine_box,
            "PesquisaClimaEncerrada": self._survey,
        }

    async def handle(self, raw: dict[str, Any]) -> bool:
        """True se o evento alterou algum read model; False se ignorado ou duplicado."""
        try:
            envelope = Envelope.model_validate(raw)
        except ValidationError as exc:
            raise InvalidEventError(f"envelope inválido: {exc.error_count()} erro(s)") from exc
        handler = self._handlers.get(envelope.event_type)
        if handler is None or envelope.event_version != 1:
            return False
        if not await self._repo.claim_event(envelope.tenant_id, envelope.event_id):
            log.info("evento duplicado ignorado", extra={"event_id": str(envelope.event_id)})
            return False
        try:
            await handler(envelope)
        except ValidationError as exc:
            raise InvalidEventError(
                f"{envelope.event_type} fora do contrato: {exc.error_count()} erro(s)"
            ) from exc
        return True

    async def _hired(self, e: Envelope) -> None:
        d = CandidatoContratadoData.model_validate(e.data)
        # O evento não traz área nem demografia (minimização); ficam como não informado.
        await self._repo.upsert_employment(
            EmploymentRecord(
                tenant_id=e.tenant_id, employee_id=d.application_id, hired_on=d.start_date, unit=d.unit
            )
        )

    async def _assessment(self, e: Envelope) -> None:
        d = AvaliacaoConcluidaData.model_validate(e.data)
        gaps = tuple(CompetencyGap(g.competency, g.gap) for g in d.competency_gaps)
        await self._repo.save_assessment(AssessmentFact(e.tenant_id, d.employee_id, d.cycle_id, gaps))

    async def _nine_box(self, e: Envelope) -> None:
        d = NineBoxCalibradoData.model_validate(e.data)
        await self._repo.save_nine_box(NineBoxFact(e.tenant_id, d.employee_id, d.cycle_id, d.quadrant))

    async def _survey(self, e: Envelope) -> None:
        d = PesquisaClimaEncerradaData.model_validate(e.data)
        segments = tuple(SurveySegment(s.segment_key, s.respondents, s.scores) for s in d.segments)
        await self._repo.save_survey(SurveyFact(e.tenant_id, d.survey_id, d.min_group_size, segments))
