import json
from pathlib import Path
from uuid import UUID

import jsonschema
import pytest
from referencing import Registry, Resource

from analytics_ai.app.ingestion import EventIngestor, InvalidEventError
from conftest import TENANT_A, TENANT_B, fixture

SCHEMAS = Path(__file__).resolve().parents[3] / "contracts" / "asyncapi" / "schemas"
NAMES = ["candidato-contratado", "avaliacao-concluida", "nine-box-calibrado", "pesquisa-clima-encerrada"]
DIRECTORY = ["colaborador-atualizado", "colaborador-desligado", "tenant-criado"]


def _registry() -> Registry:
    reg = Registry()
    for f in SCHEMAS.glob("*.json"):
        doc = json.loads(f.read_text())
        reg = reg.with_resource(f.name, Resource.from_contents(doc))
        reg = reg.with_resource(doc["$id"], Resource.from_contents(doc))
    return reg


@pytest.mark.parametrize("name", NAMES + DIRECTORY)
def test_fixtures_match_contract_schemas(name):
    schema = json.loads((SCHEMAS / f"{name}.v1.json").read_text())
    jsonschema.validate(fixture(name), schema, registry=_registry())


async def test_hire_creates_employment_and_is_idempotent(repo):
    ing = EventIngestor(repo)
    assert await ing.handle(fixture("candidato-contratado")) is True
    assert await ing.handle(fixture("candidato-contratado")) is False
    records = await repo.list_employment(TENANT_A)
    assert len(records) == 1 and records[0].unit == "Matriz"


async def test_all_events_project_to_read_models(repo):
    ing = EventIngestor(repo)
    for name in NAMES:
        assert await ing.handle(fixture(name))
    emp = UUID("55555555-5555-4555-8555-555555555551")
    assessment = await repo.latest_assessment(TENANT_A, emp)
    assert assessment and assessment.gaps[0].competency == "Comunicação"
    assert len(await repo.list_nine_box(TENANT_A, UUID("22222222-2222-4222-8222-222222222222"))) == 1
    assert await repo.get_survey(TENANT_A, UUID("66666666-6666-4666-8666-666666666661"))


async def test_read_models_do_not_leak_across_tenants(repo):
    await EventIngestor(repo).handle(fixture("avaliacao-concluida"))
    emp = UUID("55555555-5555-4555-8555-555555555551")
    assert await repo.latest_assessment(TENANT_B, emp) is None
    assert await repo.list_employment(TENANT_B) == []


async def test_invalid_payload_is_rejected(repo):
    bad = fixture("nine-box-calibrado")
    bad["data"]["quadrant"] = "Q10"
    with pytest.raises(InvalidEventError):
        await EventIngestor(repo).handle(bad)
    with pytest.raises(InvalidEventError):
        await EventIngestor(repo).handle({"nope": 1})


async def test_unknown_event_type_is_ignored(repo):
    other = fixture("candidato-contratado")
    other["event_type"] = "ElogioPublicado"
    assert await EventIngestor(repo).handle(other) is False


async def test_directory_events_build_employment_with_termination(repo):
    ing = EventIngestor(repo)
    assert await ing.handle(fixture("colaborador-atualizado")) is True
    assert await ing.handle(fixture("colaborador-desligado")) is True
    (record,) = await repo.list_employment(TENANT_A)
    assert (record.area, record.unit, record.age_band) == ("Tecnologia", "Matriz", "25-34")
    assert record.hired_on.isoformat() == "2024-03-04"
    assert record.terminated_on.isoformat() == "2026-08-20"


async def test_update_after_termination_keeps_termination(repo):
    ing = EventIngestor(repo)
    await ing.handle(fixture("colaborador-desligado"))
    later = fixture("colaborador-atualizado")
    later["event_id"] = "00000000-0000-4000-8000-0000000000a3"
    await ing.handle(later)
    (record,) = await repo.list_employment(TENANT_A)
    assert record.terminated_on is not None and record.hired_on.isoformat() == "2024-03-04"


async def test_directory_event_rejects_personal_fields(repo):
    bad = fixture("colaborador-atualizado")
    bad["data"]["birth_year"] = 1990
    with pytest.raises(InvalidEventError):
        await EventIngestor(repo).handle(bad)


async def test_tenant_created_stores_settings_once_and_per_tenant(repo):
    ing = EventIngestor(repo)
    assert await ing.handle(fixture("tenant-criado")) is True
    assert await ing.handle(fixture("tenant-criado")) is False
    settings = await repo.get_tenant_settings(TENANT_A)
    assert settings and settings.analytics_enabled and not settings.ai_enabled
    assert await repo.get_tenant_settings(TENANT_B) is None


class _FlakyNineBox:
    """Repositório que falha uma vez ao gravar o nine box, como um banco que cai no meio da transação."""

    def __init__(self, inner) -> None:
        self.inner = inner
        self.failures_left = 1

    def unit_of_work(self, tenant_id):
        return self.inner.unit_of_work(tenant_id)

    def __getattr__(self, name):
        return getattr(self.inner, name)


async def test_effect_failure_rolls_back_the_claim_so_redelivery_applies_the_event(repo, monkeypatch):
    flaky = _FlakyNineBox(repo)
    original = repo.save_nine_box

    async def failing_once(fact):
        if flaky.failures_left:
            flaky.failures_left -= 1
            raise ConnectionError("banco fora do ar")
        await original(fact)

    monkeypatch.setattr(repo, "save_nine_box", failing_once)
    ing = EventIngestor(flaky)
    event = fixture("nine-box-calibrado")
    cycle = UUID(event["data"]["cycle_id"])

    with pytest.raises(ConnectionError):
        await ing.handle(event)
    assert await repo.list_nine_box(TENANT_A, cycle) == []

    # A reentrega do Kafka não é tratada como duplicata: o registro de "processado" foi desfeito junto.
    assert await ing.handle(event) is True
    assert len(await repo.list_nine_box(TENANT_A, cycle)) == 1
    assert await ing.handle(event) is False


async def test_the_same_event_id_in_another_tenant_is_not_a_duplicate(repo):
    ing = EventIngestor(repo)
    first = fixture("candidato-contratado")
    other = {**first, "tenant_id": str(TENANT_B)}
    assert await ing.handle(first) is True
    assert await ing.handle(other) is True
    assert len(await repo.list_employment(TENANT_B)) == 1


def _erasure(tenant, subject, event_id):
    return {
        "event_id": str(event_id),
        "event_type": "TitularExclusaoSolicitada",
        "event_version": 1,
        "tenant_id": str(tenant),
        "aggregate_id": str(subject),
        "occurred_at": "2026-11-22T12:00:00Z",
        "producer": "tenant-service",
        "data": {"request_id": "77777777-7777-4777-8777-777777777777", "subject_id": str(subject)},
    }


async def test_erasure_removes_the_subject_only_in_the_event_tenant_and_is_idempotent(repo):
    from uuid import uuid4

    ing = EventIngestor(repo)
    nine_box = fixture("nine-box-calibrado")
    employee = UUID(nine_box["data"]["employee_id"])
    cycle = UUID(nine_box["data"]["cycle_id"])
    assert await ing.handle(nine_box) is True
    assert await ing.handle(fixture("avaliacao-concluida")) is True
    assert await ing.handle({**nine_box, "event_id": str(uuid4()), "tenant_id": str(TENANT_B)}) is True

    event_id = uuid4()
    assert await ing.handle(_erasure(TENANT_A, employee, event_id)) is True
    assert await repo.list_nine_box(TENANT_A, cycle) == []
    assert await repo.latest_assessment(TENANT_A, employee) is None
    # O mesmo employee_id no tenant B é outra pessoa para este tenant: o evento do A não o alcança.
    assert len(await repo.list_nine_box(TENANT_B, cycle)) == 1

    assert await ing.handle(_erasure(TENANT_A, employee, event_id)) is False  # reentrega


async def test_erasure_event_fora_do_contrato_is_invalid(repo):
    from uuid import uuid4

    bad = _erasure(TENANT_A, uuid4(), uuid4())
    bad["data"] = {"subject_id": "nao-e-uuid"}
    with pytest.raises(InvalidEventError):
        await EventIngestor(repo).handle(bad)


async def test_late_event_after_erasure_does_not_resurrect_the_subject(repo):
    from uuid import uuid4

    ing = EventIngestor(repo)
    nine_box = fixture("nine-box-calibrado")
    employee = UUID(nine_box["data"]["employee_id"])
    cycle = UUID(nine_box["data"]["cycle_id"])

    # A exclusão chega ANTES dos dados (não há ordem entre tópicos).
    assert await ing.handle(_erasure(TENANT_A, employee, uuid4())) is True
    assert await ing.handle(nine_box) is True  # consumido, mas ignorado pelo projetor
    assert await ing.handle(fixture("avaliacao-concluida")) is True
    assert await ing.handle(fixture("candidato-contratado")) is True  # outra pessoa: continua projetando
    assert await repo.list_nine_box(TENANT_A, cycle) == []
    assert await repo.latest_assessment(TENANT_A, employee) is None
    assert len(await repo.list_employment(TENANT_A)) == 1
