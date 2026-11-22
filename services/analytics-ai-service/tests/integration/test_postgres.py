"""Read models contra um Postgres real, com RLS e o papel da aplicação sem BYPASSRLS (ADR 0002).

Exige ANALYTICS_PG_ADMIN_URL (um superusuário, ex.: postgresql://postgres:postgres@localhost:5432/postgres) e roda no CI.
Cada execução cria o próprio banco e os próprios papéis.
"""

from __future__ import annotations

import os
import uuid
from dataclasses import replace
from datetime import date
from urllib.parse import urlsplit, urlunsplit

import asyncpg
import pytest

from analytics_ai.adapters.migrate import migrate
from analytics_ai.adapters.postgres import PostgresRepository
from analytics_ai.app.ingestion import EventIngestor
from analytics_ai.domain.models import (
    AssessmentFact,
    CompetencyGap,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    SurveySegment,
    TenantSettings,
)
from conftest import TENANT_A, TENANT_B, fixture

ADMIN = os.environ.get("ANALYTICS_PG_ADMIN_URL")
pytestmark = pytest.mark.skipif(not ADMIN, reason="defina ANALYTICS_PG_ADMIN_URL com um Postgres disponível")


def _with(url: str, *, user: str | None = None, password: str | None = None, db: str | None = None) -> str:
    parts = urlsplit(url)
    host = parts.hostname or "localhost"
    port = f":{parts.port}" if parts.port else ""
    creds = (
        f"{user}:{password}@" if user else (f"{parts.username}:{parts.password}@" if parts.username else "")
    )
    return urlunsplit((parts.scheme, f"{creds}{host}{port}", f"/{db}" if db else parts.path, "", ""))


@pytest.fixture
async def repo():
    assert ADMIN
    suffix = uuid.uuid4().hex[:8]
    db, owner, login = f"analytics_{suffix}", f"owner_{suffix}", f"app_{suffix}"
    admin = await asyncpg.connect(ADMIN)
    await admin.execute(f"CREATE ROLE {owner} LOGIN PASSWORD 'ci'")
    await admin.execute(f"CREATE DATABASE {db} OWNER {owner}")
    # Como no infra/postgres/init: o papel da aplicação já existe, porque o dono do esquema não tem CREATEROLE.
    await admin.execute(
        "DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'analytics_app')"
        " THEN CREATE ROLE analytics_app NOLOGIN; END IF; END $$"
    )
    await admin.close()

    await migrate(_with(ADMIN, user=owner, password="ci", db=db))
    assert await migrate(_with(ADMIN, user=owner, password="ci", db=db)) == []  # idempotente

    admin = await asyncpg.connect(_with(ADMIN, db=db))
    await admin.execute(f"CREATE ROLE {login} LOGIN PASSWORD 'ci' IN ROLE analytics_app")
    await admin.close()

    repo = PostgresRepository(_with(ADMIN, user=login, password="ci", db=db))
    await repo.start()
    yield repo
    await repo.close()


def _employment(tenant: uuid.UUID, employee: uuid.UUID) -> EmploymentRecord:
    return EmploymentRecord(
        tenant_id=tenant, employee_id=employee, hired_on=date(2025, 3, 1), area="obras", unit="Matriz"
    )


async def _seed(repo: PostgresRepository, tenant: uuid.UUID) -> uuid.UUID:
    employee, cycle, survey = uuid.uuid4(), uuid.uuid4(), uuid.uuid4()
    await repo.save_tenant_settings(TenantSettings(tenant, "slug", "essencial", True, False))
    await repo.upsert_employment(_employment(tenant, employee))
    await repo.save_assessment(AssessmentFact(tenant, employee, cycle, (CompetencyGap("Comunicação", 2),)))
    await repo.save_nine_box(NineBoxFact(tenant, employee, cycle, "Q8"))
    await repo.save_survey(SurveyFact(tenant, survey, 5, (SurveySegment("_all", 9, {"q1": 4.2}),)))
    await repo.claim_event(tenant, uuid.uuid4())
    return cycle


async def test_round_trip_of_every_read_model(repo):
    employee, cycle, survey = uuid.uuid4(), uuid.uuid4(), uuid.uuid4()
    record = _employment(TENANT_A, employee)
    await repo.upsert_employment(record)
    assert await repo.get_employment(TENANT_A, employee) == record
    terminated = replace(record, terminated_on=date(2026, 1, 31))
    await repo.upsert_employment(terminated)  # upsert: mesma chave, valor novo
    assert await repo.list_employment(TENANT_A) == [terminated]

    await repo.save_assessment(AssessmentFact(TENANT_A, employee, cycle, (CompetencyGap("Entrega", 3),)))
    assert (await repo.latest_assessment(TENANT_A, employee)).gaps == (CompetencyGap("Entrega", 3),)

    await repo.save_nine_box(NineBoxFact(TENANT_A, employee, cycle, "Q9"))
    await repo.save_nine_box(NineBoxFact(TENANT_A, employee, cycle, "Q8"))  # upsert
    assert [f.quadrant for f in await repo.list_nine_box(TENANT_A, cycle)] == ["Q8"]

    await repo.save_survey(SurveyFact(TENANT_A, survey, 5, (SurveySegment("_all", 9, {"q1": 4.25}),)))
    assert (await repo.get_survey(TENANT_A, survey)).segments[0].scores == {"q1": 4.25}

    settings = TenantSettings(TENANT_A, "horizonte", "crescimento", True, True)
    await repo.save_tenant_settings(settings)
    assert await repo.get_tenant_settings(TENANT_A) == settings


async def test_claim_is_per_tenant_and_idempotent(repo):
    event = uuid.uuid4()
    assert await repo.claim_event(TENANT_A, event) is True
    assert await repo.claim_event(TENANT_A, event) is False
    assert await repo.claim_event(TENANT_B, event) is True


async def test_tenant_b_reads_nothing_of_tenant_a_through_the_repository(repo):
    cycle = await _seed(repo, TENANT_A)
    employee = (await repo.list_employment(TENANT_A))[0].employee_id
    assert await repo.list_employment(TENANT_B) == []
    assert await repo.get_employment(TENANT_B, employee) is None
    assert await repo.latest_assessment(TENANT_B, employee) is None
    assert await repo.list_nine_box(TENANT_B, cycle) == []
    assert await repo.get_tenant_settings(TENANT_B) is None


async def test_rls_blocks_cross_tenant_access_even_without_where(repo):
    await _seed(repo, TENANT_A)
    await _seed(repo, TENANT_B)
    tables = [
        "processed_events",
        "tenant_settings",
        "employment_records",
        "assessment_facts",
        "nine_box_facts",
        "survey_facts",
    ]
    pool = repo._pool
    for table in tables:
        async with pool.acquire() as conn, conn.transaction():
            await conn.execute("SELECT set_config('app.tenant_id', $1, true)", str(TENANT_A))
            seen = {r["tenant_id"] for r in await conn.fetch(f"SELECT tenant_id FROM {table}")}
            assert seen == {TENANT_A}, f"{table} vazou {seen}"
        assert await pool.fetchval(f"SELECT count(*) FROM {table}") == 0, (
            f"{table} sem escopo deveria ser invisível"
        )

    async with pool.acquire() as conn, conn.transaction():
        await conn.execute("SELECT set_config('app.tenant_id', $1, true)", str(TENANT_A))
        with pytest.raises(asyncpg.PostgresError):
            await conn.execute(
                "INSERT INTO nine_box_facts (tenant_id, cycle_id, employee_id, quadrant) VALUES ($1, $2, $3, 'Q1')",
                TENANT_B,
                uuid.uuid4(),
                uuid.uuid4(),
            )
    async with pool.acquire() as conn, conn.transaction():
        await conn.execute("SELECT set_config('app.tenant_id', $1, true)", str(TENANT_A))
        tag = await conn.execute(
            "UPDATE employment_records SET area = 'invadida' WHERE tenant_id = $1", TENANT_B
        )
        assert tag == "UPDATE 0"


async def test_scope_does_not_survive_the_transaction(repo):
    await _seed(repo, TENANT_A)
    async with repo._pool.acquire() as conn:
        async with conn.transaction():
            await conn.execute("SELECT set_config('app.tenant_id', $1, true)", str(TENANT_A))
        assert await conn.fetchval("SELECT count(*) FROM employment_records") == 0


async def test_ingestor_deduplicates_and_a_failure_after_the_claim_rolls_it_back(repo):
    ing = EventIngestor(repo)
    event = fixture("candidato-contratado")
    assert await ing.handle(event) is True
    assert await ing.handle(event) is False
    assert len(await repo.list_employment(TENANT_A)) == 1

    # Falha depois do claim desfaz o claim: a reentrega aplica o evento em vez de descartá-lo como duplicata.
    event_id = uuid.uuid4()
    with pytest.raises(RuntimeError):
        async with repo.unit_of_work(TENANT_A) as tx:
            assert await tx.claim_event(TENANT_A, event_id) is True
            raise RuntimeError("falha depois do claim")
    assert await repo.claim_event(TENANT_A, event_id) is True
