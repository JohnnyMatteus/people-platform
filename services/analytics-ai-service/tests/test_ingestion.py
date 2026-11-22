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
DIRECTORY = ["colaborador-atualizado", "colaborador-desligado"]


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
