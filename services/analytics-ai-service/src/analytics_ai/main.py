"""Composição: lê a configuração e liga os adaptadores."""

from __future__ import annotations

import asyncio
import contextlib
import logging
from collections.abc import AsyncIterator

import uvicorn
from fastapi import FastAPI

from analytics_ai.adapters.http.api import Deps, create_app
from analytics_ai.adapters.http.auth import JwksVerifier
from analytics_ai.adapters.kafka_consumer import consume
from analytics_ai.adapters.openai_llm import OpenAICompatibleClient
from analytics_ai.adapters.postgres import PostgresRepository
from analytics_ai.app.ai.budget import InMemoryAiBudget
from analytics_ai.app.ai.runner import AiRunner
from analytics_ai.app.ai.service import AiService
from analytics_ai.app.analytics_service import AnalyticsService
from analytics_ai.app.ingestion import EventIngestor
from analytics_ai.config import Settings


def build_app(settings: Settings) -> FastAPI:
    repo = PostgresRepository(settings.database_url)
    llm = OpenAICompatibleClient(
        base_url=settings.llm_base_url,
        api_key=settings.llm_api_key,
        model=settings.llm_model,
        timeout_s=settings.llm_timeout_s,
    )
    budget = InMemoryAiBudget(
        daily_calls_per_resource=settings.ai_daily_calls_per_resource, daily_tokens=settings.ai_daily_tokens
    )
    runner = AiRunner(llm, budget, max_tokens=settings.llm_max_tokens)
    deps = Deps(
        verifier=JwksVerifier(
            jwks_url=settings.oidc_jwks_url, issuer=settings.oidc_issuer, audience=settings.oidc_audience
        ),
        analytics=AnalyticsService(repo, settings.min_group_size),
        ai=AiService(runner, repo),
        model_name=settings.llm_model,
        min_group_size=settings.min_group_size,
    )
    app = create_app(deps)

    @contextlib.asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[None]:
        await repo.start()
        task = None
        if settings.kafka_brokers:
            task = asyncio.create_task(
                consume(
                    brokers=settings.kafka_brokers,
                    group_id=settings.kafka_group_id,
                    topics=settings.kafka_topics.split(","),
                    ingestor=EventIngestor(repo),
                )
            )
        try:
            yield
        finally:
            if task:
                task.cancel()
            await repo.close()

    app.router.lifespan_context = lifespan
    return app


def run() -> None:
    logging.basicConfig(
        level=logging.INFO, format='{"level":"%(levelname)s","logger":"%(name)s","msg":"%(message)s"}'
    )
    settings = Settings()
    uvicorn.run(build_app(settings), host="0.0.0.0", port=settings.http_addr_port)  # noqa: S104


if __name__ == "__main__":
    run()
