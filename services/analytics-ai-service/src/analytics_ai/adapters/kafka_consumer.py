"""Consumidor Kafka: entrega cada mensagem ao EventIngestor. Offset só avança depois do efeito gravado."""

from __future__ import annotations

import asyncio
import json
import logging
from collections.abc import Awaitable, Callable
from typing import Any, Protocol

from aiokafka import AIOKafkaConsumer, TopicPartition

from analytics_ai.app.ingestion import EventIngestor, InvalidEventError

log = logging.getLogger(__name__)
RETRY_DELAY_S = 2.0


class Message(Protocol):
    topic: str
    partition: int
    offset: int
    value: bytes | None


async def process(
    message: Message,
    ingestor: EventIngestor,
    *,
    rewind: Callable[[TopicPartition, int], Awaitable[None] | None],
    commit: Callable[[], Awaitable[None]],
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> None:
    """Trata uma mensagem. Inválida: descarta com log e avança (não pode travar a partição). Falha transitória:
    volta o offset para esta mensagem e NÃO confirma, então ela é reentregue em vez de perdida."""
    try:
        payload: dict[str, Any] = json.loads(message.value or b"{}")
        await ingestor.handle(payload)
    except (InvalidEventError, json.JSONDecodeError) as exc:
        log.error(
            "evento descartado",
            extra={"topic": message.topic, "offset": message.offset, "reason": str(exc)},
        )
    except Exception:
        log.exception(
            "falha ao processar, a mensagem será reentregue",
            extra={"topic": message.topic, "offset": message.offset},
        )
        result = rewind(TopicPartition(message.topic, message.partition), message.offset)
        if result is not None:
            await result
        await sleep(RETRY_DELAY_S)
        return
    await commit()


async def consume(*, brokers: str, group_id: str, topics: list[str], ingestor: EventIngestor) -> None:
    consumer = AIOKafkaConsumer(
        *topics, bootstrap_servers=brokers, group_id=group_id, enable_auto_commit=False
    )
    await consumer.start()
    try:
        async for message in consumer:
            await process(message, ingestor, rewind=consumer.seek, commit=consumer.commit)
    finally:
        await consumer.stop()
