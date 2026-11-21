"""Cliente compatível com /v1/chat/completions. base_url, modelo e chave são configuráveis."""

from __future__ import annotations

from dataclasses import dataclass

import openai
from openai import AsyncOpenAI

from analytics_ai.app.ai.runner import AiProviderError


@dataclass(frozen=True, slots=True)
class Completion:
    text: str
    total_tokens: int


class OpenAICompatibleClient:
    def __init__(self, *, base_url: str, api_key: str, model: str, timeout_s: float) -> None:
        self._client = AsyncOpenAI(base_url=base_url, api_key=api_key, timeout=timeout_s, max_retries=0)
        self.model = model

    async def complete(self, *, system: str, user: str, max_tokens: int) -> Completion:
        try:
            response = await self._client.chat.completions.create(
                model=self.model,
                max_tokens=max_tokens,
                temperature=0.2,
                response_format={"type": "json_object"},
                messages=[{"role": "system", "content": system}, {"role": "user", "content": user}],
            )
        except openai.OpenAIError as exc:
            raise AiProviderError(type(exc).__name__) from exc
        text = response.choices[0].message.content or ""
        tokens = response.usage.total_tokens if response.usage else 0
        return Completion(text, tokens)
