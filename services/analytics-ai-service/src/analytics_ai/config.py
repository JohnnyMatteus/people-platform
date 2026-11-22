from __future__ import annotations

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Falha cedo: sem as variáveis obrigatórias o serviço não sobe."""

    model_config = SettingsConfigDict(env_file=None, extra="ignore")

    http_addr_port: int = Field(8000, alias="HTTP_PORT")
    oidc_issuer: str = Field(alias="OIDC_ISSUER")
    oidc_jwks_url: str = Field(alias="OIDC_JWKS_URL")
    oidc_audience: str = Field("people-api", alias="OIDC_AUDIENCE")

    kafka_brokers: str = Field("", alias="KAFKA_BROKERS")
    kafka_group_id: str = Field("analytics-ai-service", alias="KAFKA_GROUP_ID")
    kafka_topics: str = Field(
        "people.performance.events.v1,people.recruitment.events.v1,people.engagement.events.v1",
        alias="KAFKA_TOPICS",
    )

    min_group_size: int = Field(5, ge=3, alias="MIN_GROUP_SIZE")

    llm_base_url: str = Field("https://api.openai.com/v1", alias="LLM_BASE_URL")
    llm_api_key: str = Field(alias="LLM_API_KEY")
    llm_model: str = Field("gpt-4o-mini", alias="LLM_MODEL")
    llm_timeout_s: float = Field(30.0, gt=0, alias="LLM_TIMEOUT_S")
    llm_max_tokens: int = Field(800, ge=64, le=4000, alias="LLM_MAX_TOKENS")
    ai_daily_calls_per_resource: int = Field(50, ge=1, alias="AI_DAILY_CALLS_PER_RESOURCE")
    ai_daily_tokens: int = Field(200_000, ge=1000, alias="AI_DAILY_TOKENS")
