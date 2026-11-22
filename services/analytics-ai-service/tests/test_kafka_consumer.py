import json
from dataclasses import dataclass

from analytics_ai.adapters.kafka_consumer import process
from analytics_ai.app.ingestion import EventIngestor
from conftest import TENANT_A, fixture


@dataclass
class Msg:
    value: bytes | None
    topic: str = "people.performance.events.v1"
    partition: int = 2
    offset: int = 41


class Recorder:
    def __init__(self) -> None:
        self.commits = 0
        self.rewinds: list[tuple[str, int, int]] = []
        self.sleeps: list[float] = []

    async def commit(self) -> None:
        self.commits += 1

    def rewind(self, tp, offset) -> None:
        self.rewinds.append((tp.topic, tp.partition, offset))

    async def sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)


async def _run(msg, ingestor, rec):
    await process(msg, ingestor, rewind=rec.rewind, commit=rec.commit, sleep=rec.sleep)


async def test_valid_event_is_applied_and_committed(repo):
    rec = Recorder()
    await _run(Msg(json.dumps(fixture("nine-box-calibrado")).encode()), EventIngestor(repo), rec)
    assert rec.commits == 1 and not rec.rewinds
    assert (
        len(
            await repo.list_nine_box(
                TENANT_A, __import__("uuid").UUID(fixture("nine-box-calibrado")["data"]["cycle_id"])
            )
        )
        == 1
    )


async def test_poison_message_is_dropped_and_the_partition_moves_on(repo):
    rec = Recorder()
    for raw in (b"{nao-json", b'{"event_type": "x"}'):
        await _run(Msg(raw), EventIngestor(repo), rec)
    assert rec.commits == 2 and not rec.rewinds


async def test_transient_failure_rewinds_without_committing(repo, monkeypatch):
    async def boom(_fact):
        raise ConnectionError("banco fora do ar")

    monkeypatch.setattr(repo, "save_nine_box", boom)
    rec = Recorder()
    await _run(Msg(json.dumps(fixture("nine-box-calibrado")).encode()), EventIngestor(repo), rec)
    assert rec.commits == 0
    assert rec.rewinds == [("people.performance.events.v1", 2, 41)]
    assert rec.sleeps
