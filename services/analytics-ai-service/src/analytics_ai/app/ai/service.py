"""Casos de uso de IA. Cada um monta o prompt com o mínimo necessário e delimita conteúdo externo."""

from __future__ import annotations

from uuid import UUID

from analytics_ai.app.ports import ReadModelRepository

from .guard import Pseudonymizer, wrap_external
from .runner import AiRunner
from .schemas import (
    ClimateActionPlan,
    FeedbackDraft,
    FeedbackDraftRequest,
    IdpSuggestions,
    OneOnOneBrief,
    OneOnOneBriefRequest,
)


class NotFoundError(Exception):
    pass


class AiService:
    def __init__(self, runner: AiRunner, repo: ReadModelRepository) -> None:
        self._runner = runner
        self._repo = repo

    async def draft_feedback(self, tenant_id: UUID, req: FeedbackDraftRequest) -> tuple[FeedbackDraft, str]:
        names = Pseudonymizer()
        who = names.alias(req.recipient_name)
        topics = names.mask("\n".join(f"- {t}" for t in req.topics))
        user = (
            f"Redija um rascunho de feedback para {who} no tom {req.tone.value}, "
            f"com base nos tópicos abaixo.\n{wrap_external('topicos', topics)}"
        )
        gen = await self._runner.run(
            tenant_id=tenant_id,
            resource="feedback_draft",
            prompt_version="feedback-v1",
            user=user,
            schema=FeedbackDraft,
        )
        out = gen.output
        restored = out.model_copy(
            update={
                "draft": names.restore(out.draft),
                "improvement_tips": [names.restore(t) for t in out.improvement_tips],
            }
        )
        return restored, gen.prompt_version

    async def suggest_idp(self, tenant_id: UUID, employee_id: UUID) -> tuple[IdpSuggestions, str]:
        fact = await self._repo.latest_assessment(tenant_id, employee_id)
        if fact is None or not fact.gaps:
            raise NotFoundError("avaliação com gaps não encontrada")
        # Sem identidade no prompt: só competência e tamanho do gap.
        gaps = "\n".join(f"- {g.competency}: gap {g.gap}" for g in fact.gaps)
        user = f"Sugira ações de PDI para os gaps de competência abaixo.\n{wrap_external('gaps', gaps)}"
        gen = await self._runner.run(
            tenant_id=tenant_id,
            resource="idp_suggestions",
            prompt_version="idp-v1",
            user=user,
            schema=IdpSuggestions,
        )
        return gen.output, gen.prompt_version

    async def draft_climate_plan(self, tenant_id: UUID, survey_id: UUID) -> tuple[ClimateActionPlan, str]:
        survey = await self._repo.get_survey(tenant_id, survey_id)
        if survey is None:
            raise NotFoundError("pesquisa não encontrada")
        # Defesa em profundidade: reaplica o k mesmo que o produtor já tenha filtrado.
        rows = [
            f"- {s.segment_key} ({s.respondents} respondentes): "
            + ", ".join(f"{dim}={score}" for dim, score in sorted(s.scores.items()))
            for s in survey.segments
            if s.respondents >= survey.min_group_size
        ]
        if not rows:
            raise NotFoundError("nenhum segmento atinge o tamanho mínimo")
        user = (
            "Proponha um plano de ação para melhorar o clima a partir dos resultados agregados abaixo.\n"
            + wrap_external("clima", "\n".join(rows))
        )
        gen = await self._runner.run(
            tenant_id=tenant_id,
            resource="climate_plan",
            prompt_version="climate-v1",
            user=user,
            schema=ClimateActionPlan,
        )
        return gen.output, gen.prompt_version

    async def one_on_one_brief(self, tenant_id: UUID, req: OneOnOneBriefRequest) -> tuple[OneOnOneBrief, str]:
        names = Pseudonymizer()
        who = names.alias(req.employee_name)

        def block(label: str, items: list[str]) -> str:
            return wrap_external(label, names.mask("\n".join(f"- {i}" for i in items) or "(vazio)"))

        user = (
            f"Prepare o contexto para a próxima 1:1 com {who}.\n"
            f"Pauta anterior:\n{block('pauta_anterior', req.previous_agenda_items)}\n"
            f"Feedbacks recentes:\n{block('feedbacks', req.recent_feedback_summaries)}\n"
            f"Elogios recentes:\n{block('elogios', req.recent_praises)}"
        )
        gen = await self._runner.run(
            tenant_id=tenant_id,
            resource="one_on_one_brief",
            prompt_version="one-on-one-v1",
            user=user,
            schema=OneOnOneBrief,
        )
        out = gen.output
        restored = OneOnOneBrief(
            talking_points=[names.restore(t) for t in out.talking_points],
            follow_ups=[names.restore(t) for t in out.follow_ups],
            praise_to_mention=[names.restore(t) for t in out.praise_to_mention],
        )
        return restored, gen.prompt_version
