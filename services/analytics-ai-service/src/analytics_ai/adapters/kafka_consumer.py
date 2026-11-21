"""Consumidor Kafka: entrega cada mensagem ao EventIngestor. Offset só avança após processar."""

from __future__ import annotations

import json
import logging

from aiokafka import AIOKafkaConsumer

from analytics_ai.app.ingestion import EventIngestor, InvalidEventError

log = logging.getLogger(__name__)


async def consume(*, brokers: str, group_id: str, topics: list[str], ingestor: EventIngestor) -> None:
    consumer = AIOKafkaConsumer(
        *topics, bootstrap_servers=brokers, group_id=group_id, enable_auto_commit=False
    )
    await consumer.start()
    try:
        async for message in consumer:
            try:
                await ingestor.handle(json.loads(message.value or b"{}"))
            except (InvalidEventError, json.JSONDecodeError) as exc:
                # Mensagem venenosa não pode travar a partição; vai para o log e segue.
                log.error(
                    "evento descartado",
                    extra={"topic": message.topic, "offset": message.offset, "reason": str(exc)},
                )
            await consumer.commit()
    finally:
        await consumer.stop()
