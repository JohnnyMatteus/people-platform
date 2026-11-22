"""Projeta eventos Kafka nos read models. Idempotente por event_id."""

from __future__ import annotations

import logging
from collections.abc import Awaitable, Callable
from typing import Any
from uuid import UUID

from pydantic import ValidationError

from analytics_ai.domain.models import (
    AssessmentFact,
    CompetencyGap,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    SurveySegment,
    TenantSettings,
)

from .events import (
    AvaliacaoConcluidaData,
    CandidatoContratadoData,
    ColaboradorAtualizadoData,
    ColaboradorDesligadoData,
    Envelope,
    NineBoxCalibradoData,
    PesquisaClimaEncerradaData,
    TenantCriadoData,
    TitularExclusaoSolicitadaData,
)
from .ports import ReadModelRepository, UnitOfWork

log = logging.getLogger(__name__)


class InvalidEventError(ValueError):
    """Evento fora do contrato. Não adianta tentar de novo."""


class EventIngestor:
    def __init__(self, uow: UnitOfWork) -> None:
        self._uow = uow
        self._handlers: dict[str, Callable[[ReadModelRepository, Envelope], Awaitable[None]]] = {
            "TenantCriado": self._tenant_created,
            "CandidatoContratado": self._hired,
            "ColaboradorAtualizado": self._updated,
            "ColaboradorDesligado": self._terminated,
            "AvaliacaoConcluida": self._assessment,
            "NineBoxCalibrado": self._nine_box,
            "PesquisaClimaEncerrada": self._survey,
            "TitularExclusaoSolicitada": self._erase_subject,
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
        # O tenant do efeito é o do envelope; claim e efeito saem na mesma transação do tenant.
        async with self._uow.unit_of_work(envelope.tenant_id) as repo:
            if not await repo.claim_event(envelope.tenant_id, envelope.event_id):
                log.info("evento duplicado ignorado", extra={"event_id": str(envelope.event_id)})
                return False
            try:
                await handler(repo, envelope)
            except ValidationError as exc:
                raise InvalidEventError(
                    f"{envelope.event_type} fora do contrato: {exc.error_count()} erro(s)"
                ) from exc
        return True

    async def _tenant_created(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = TenantCriadoData.model_validate(e.data)
        await repo.save_tenant_settings(
            TenantSettings(
                tenant_id=e.tenant_id,
                slug=d.slug,
                plan=d.plan,
                analytics_enabled="analytics" in d.features,
                ai_enabled="ai_assist" in d.features,
            )
        )

    async def _hired(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = CandidatoContratadoData.model_validate(e.data)
        if await self._ignored(repo, e, d.application_id):
            return
        # O evento não traz área nem demografia (minimização); ficam como não informado.
        await repo.upsert_employment(
            EmploymentRecord(
                tenant_id=e.tenant_id, employee_id=d.application_id, hired_on=d.start_date, unit=d.unit
            )
        )

    async def _updated(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = ColaboradorAtualizadoData.model_validate(e.data)
        if await self._ignored(repo, e, d.employee_id):
            return
        current = await repo.get_employment(e.tenant_id, d.employee_id)
        await repo.upsert_employment(
            EmploymentRecord(
                tenant_id=e.tenant_id,
                employee_id=d.employee_id,
                hired_on=d.hired_on,
                area=d.area,
                unit=d.unit,
                age_band=d.age_band,
                terminated_on=current.terminated_on if current else None,
            )
        )

    async def _terminated(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = ColaboradorDesligadoData.model_validate(e.data)
        if await self._ignored(repo, e, d.employee_id):
            return
        current = await repo.get_employment(e.tenant_id, d.employee_id)
        # Desligamento chegando antes da atualização (ordem não garantida entre tópicos) cria o vínculo
        # com a data de desligamento como admissão provisória; a próxima atualização corrige hired_on.
        await repo.upsert_employment(
            EmploymentRecord(
                tenant_id=e.tenant_id,
                employee_id=d.employee_id,
                hired_on=current.hired_on if current else d.terminated_on,
                area=d.area,
                unit=d.unit,
                age_band=d.age_band,
                terminated_on=d.terminated_on,
            )
        )

    async def _assessment(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = AvaliacaoConcluidaData.model_validate(e.data)
        if await self._ignored(repo, e, d.employee_id):
            return
        gaps = tuple(CompetencyGap(g.competency, g.gap) for g in d.competency_gaps)
        await repo.save_assessment(AssessmentFact(e.tenant_id, d.employee_id, d.cycle_id, gaps))

    async def _nine_box(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = NineBoxCalibradoData.model_validate(e.data)
        if await self._ignored(repo, e, d.employee_id):
            return
        await repo.save_nine_box(NineBoxFact(e.tenant_id, d.employee_id, d.cycle_id, d.quadrant))

    async def _survey(self, repo: ReadModelRepository, e: Envelope) -> None:
        d = PesquisaClimaEncerradaData.model_validate(e.data)
        segments = tuple(SurveySegment(s.segment_key, s.respondents, s.scores) for s in d.segments)
        await repo.save_survey(SurveyFact(e.tenant_id, d.survey_id, d.min_group_size, segments))

    @staticmethod
    async def _ignored(repo: ReadModelRepository, e: Envelope, subject: UUID) -> bool:
        """Titular com lápide: o evento chegou depois do pedido de exclusão e não pode recriar o dado."""
        if await repo.is_erased(e.tenant_id, subject):
            log.info(
                "evento sobre titular apagado ignorado",
                extra={"event_type": e.event_type, "tenant_id": str(e.tenant_id)},
            )
            return True
        return False

    async def _erase_subject(self, repo: ReadModelRepository, e: Envelope) -> None:
        """LGPD: apaga o que o serviço projetou sobre o titular, só no tenant do evento (mesma transação do claim).

        subject_id é tratado como employee_id (o `sub` do colaborador) e também alcança o vínculo provisório criado por
        CandidatoContratado, que usa application_id como employee_id. Não há vínculo entre candidato e colaborador.

        Os agregados já publicados (turnover, nine box por quadrante) são recalculados dos read models na leitura,
        então deixam de contar a pessoa; não há cópia derivada para limpar."""
        d = TitularExclusaoSolicitadaData.model_validate(e.data)
        removed = await repo.erase_subject(e.tenant_id, d.subject_id, d.request_id)
        log.info(
            "exclusão de titular aplicada",
            extra={"request_id": str(d.request_id), "tenant_id": str(e.tenant_id), "removed": removed},
        )
