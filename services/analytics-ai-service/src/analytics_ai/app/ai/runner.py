"""Executa uma chamada de LLM: orçamento, validação por schema e uma nova tentativa."""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass
from uuid import UUID

from pydantic import BaseModel, ValidationError

from analytics_ai.app.ports import AiBudget, LLMClient

from .guard import SYSTEM_RULES

log = logging.getLogger(__name__)


class BudgetExceededError(Exception):
    pass


class AiOutputInvalidError(Exception):
    """Saída do LLM inválida mesmo após a nova tentativa (HTTP 422 ai_output_invalid)."""


class AiProviderError(Exception):
    """Provedor indisponível ou com timeout (HTTP 502)."""


@dataclass(frozen=True, slots=True)
class Generation[T: BaseModel]:
    output: T
    prompt_version: str
    attempts: int
    tokens: int


class AiRunner:
    def __init__(self, llm: LLMClient, budget: AiBudget, *, max_tokens: int = 800) -> None:
        self._llm = llm
        self._budget = budget
        self._max_tokens = max_tokens

    async def run[T: BaseModel](
        self, *, tenant_id: UUID, resource: str, prompt_version: str, user: str, schema: type[T]
    ) -> Generation[T]:
        await self._budget.reserve(tenant_id, resource)
        format_hint = f"Formato do JSON (JSON Schema): {schema.model_json_schema()}"
        system = f"{SYSTEM_RULES}\n{format_hint}"
        prompt, tokens, error = user, 0, ""
        for attempt in (1, 2):
            started = time.perf_counter()
            result = await self._llm.complete(system=system, user=prompt, max_tokens=self._max_tokens)
            tokens += result.total_tokens
            try:
                output = schema.model_validate_json(_strip_fence(result.text))
            except ValidationError as exc:
                error = str(exc)[:1500]
                # Só metadados: nunca o conteúdo gerado nem o do prompt (ADR 0008).
                log.warning(
                    "saída de IA inválida",
                    extra={"resource": resource, "attempt": attempt, "tenant_id": str(tenant_id)},
                )
                prompt = f"{user}\n\nSua resposta anterior foi recusada pela validação:\n{error}\nCorrija e responda só o JSON."
                continue
            await self._budget.record_tokens(tenant_id, tokens)
            log.info(
                "geração de IA",
                extra={
                    "resource": resource,
                    "tenant_id": str(tenant_id),
                    "prompt_version": prompt_version,
                    "tokens": tokens,
                    "latency_ms": int((time.perf_counter() - started) * 1000),
                    "attempts": attempt,
                },
            )
            return Generation(output, prompt_version, attempt, tokens)
        await self._budget.record_tokens(tenant_id, tokens)
        raise AiOutputInvalidError(resource)


def _strip_fence(text: str) -> str:
    stripped = text.strip()
    if stripped.startswith("```"):
        stripped = stripped.split("\n", 1)[-1].removesuffix("```").strip()
    return stripped
