"""Read models em Postgres com RLS por tenant. A aplicação conecta como membro de `analytics_app` (sem BYPASSRLS)."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from typing import Any
from uuid import UUID

import asyncpg

from analytics_ai.domain.models import (
    AssessmentFact,
    CompetencyGap,
    EmploymentRecord,
    NineBoxFact,
    SurveyFact,
    SurveySegment,
    TenantSettings,
)


def _employment(r: asyncpg.Record) -> EmploymentRecord:
    return EmploymentRecord(
        tenant_id=r["tenant_id"],
        employee_id=r["employee_id"],
        hired_on=r["hired_on"],
        area=r["area"],
        unit=r["unit"],
        birth_year=r["birth_year"],
        gender=r["gender"],
        terminated_on=r["terminated_on"],
        age_band=r["age_band"],
    )


class _TxRepository:
    """As operações do repositório numa conexão que já está dentro de uma transação com o escopo do tenant."""

    def __init__(self, conn: asyncpg.Connection) -> None:
        self._c = conn

    async def claim_event(self, tenant_id: UUID, event_id: UUID) -> bool:
        tag = await self._c.execute(
            "INSERT INTO processed_events (tenant_id, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
            tenant_id,
            event_id,
        )
        return bool(tag.endswith(" 1"))

    async def save_tenant_settings(self, settings: TenantSettings) -> None:
        await self._c.execute(
            "INSERT INTO tenant_settings (tenant_id, slug, plan, analytics_enabled, ai_enabled)"
            " VALUES ($1, $2, $3, $4, $5)"
            " ON CONFLICT (tenant_id) DO UPDATE SET slug = EXCLUDED.slug, plan = EXCLUDED.plan,"
            " analytics_enabled = EXCLUDED.analytics_enabled, ai_enabled = EXCLUDED.ai_enabled",
            settings.tenant_id,
            settings.slug,
            settings.plan,
            settings.analytics_enabled,
            settings.ai_enabled,
        )

    async def get_tenant_settings(self, tenant_id: UUID) -> TenantSettings | None:
        r = await self._c.fetchrow("SELECT * FROM tenant_settings WHERE tenant_id = $1", tenant_id)
        if r is None:
            return None
        return TenantSettings(r["tenant_id"], r["slug"], r["plan"], r["analytics_enabled"], r["ai_enabled"])

    async def upsert_employment(self, record: EmploymentRecord) -> None:
        await self._c.execute(
            "INSERT INTO employment_records (tenant_id, employee_id, hired_on, area, unit, birth_year, gender,"
            " terminated_on, age_band) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)"
            " ON CONFLICT (tenant_id, employee_id) DO UPDATE SET hired_on = EXCLUDED.hired_on,"
            " area = EXCLUDED.area, unit = EXCLUDED.unit, birth_year = EXCLUDED.birth_year,"
            " gender = EXCLUDED.gender, terminated_on = EXCLUDED.terminated_on, age_band = EXCLUDED.age_band",
            record.tenant_id,
            record.employee_id,
            record.hired_on,
            record.area,
            record.unit,
            record.birth_year,
            record.gender,
            record.terminated_on,
            record.age_band,
        )

    async def get_employment(self, tenant_id: UUID, employee_id: UUID) -> EmploymentRecord | None:
        r = await self._c.fetchrow(
            "SELECT * FROM employment_records WHERE tenant_id = $1 AND employee_id = $2",
            tenant_id,
            employee_id,
        )
        return _employment(r) if r else None

    async def list_employment(self, tenant_id: UUID) -> list[EmploymentRecord]:
        rows = await self._c.fetch("SELECT * FROM employment_records WHERE tenant_id = $1", tenant_id)
        return [_employment(r) for r in rows]

    async def save_assessment(self, fact: AssessmentFact) -> None:
        gaps = json.dumps([{"competency": g.competency, "gap": g.gap} for g in fact.gaps])
        await self._c.execute(
            "INSERT INTO assessment_facts (tenant_id, employee_id, cycle_id, gaps) VALUES ($1, $2, $3, $4::jsonb)"
            " ON CONFLICT (tenant_id, employee_id) DO UPDATE SET cycle_id = EXCLUDED.cycle_id, gaps = EXCLUDED.gaps",
            fact.tenant_id,
            fact.employee_id,
            fact.cycle_id,
            gaps,
        )

    async def latest_assessment(self, tenant_id: UUID, employee_id: UUID) -> AssessmentFact | None:
        r = await self._c.fetchrow(
            "SELECT * FROM assessment_facts WHERE tenant_id = $1 AND employee_id = $2", tenant_id, employee_id
        )
        if r is None:
            return None
        gaps = tuple(CompetencyGap(g["competency"], g["gap"]) for g in json.loads(r["gaps"]))
        return AssessmentFact(r["tenant_id"], r["employee_id"], r["cycle_id"], gaps)

    async def save_nine_box(self, fact: NineBoxFact) -> None:
        await self._c.execute(
            "INSERT INTO nine_box_facts (tenant_id, cycle_id, employee_id, quadrant) VALUES ($1, $2, $3, $4)"
            " ON CONFLICT (tenant_id, cycle_id, employee_id) DO UPDATE SET quadrant = EXCLUDED.quadrant",
            fact.tenant_id,
            fact.cycle_id,
            fact.employee_id,
            fact.quadrant,
        )

    async def list_nine_box(self, tenant_id: UUID, cycle_id: UUID) -> list[NineBoxFact]:
        rows = await self._c.fetch(
            "SELECT * FROM nine_box_facts WHERE tenant_id = $1 AND cycle_id = $2", tenant_id, cycle_id
        )
        return [NineBoxFact(r["tenant_id"], r["employee_id"], r["cycle_id"], r["quadrant"]) for r in rows]

    async def save_survey(self, fact: SurveyFact) -> None:
        segments = json.dumps(
            [
                {"segment_key": s.segment_key, "respondents": s.respondents, "scores": s.scores}
                for s in fact.segments
            ]
        )
        await self._c.execute(
            "INSERT INTO survey_facts (tenant_id, survey_id, min_group_size, segments)"
            " VALUES ($1, $2, $3, $4::jsonb)"
            " ON CONFLICT (tenant_id, survey_id) DO UPDATE SET min_group_size = EXCLUDED.min_group_size,"
            " segments = EXCLUDED.segments",
            fact.tenant_id,
            fact.survey_id,
            fact.min_group_size,
            segments,
        )

    async def get_survey(self, tenant_id: UUID, survey_id: UUID) -> SurveyFact | None:
        r = await self._c.fetchrow(
            "SELECT * FROM survey_facts WHERE tenant_id = $1 AND survey_id = $2", tenant_id, survey_id
        )
        if r is None:
            return None
        segments: list[dict[str, Any]] = json.loads(r["segments"])
        return SurveyFact(
            r["tenant_id"],
            r["survey_id"],
            r["min_group_size"],
            tuple(SurveySegment(s["segment_key"], s["respondents"], s["scores"]) for s in segments),
        )

    async def erase_subject(self, tenant_id: UUID, subject_id: UUID, request_id: UUID) -> int:
        # A única via de exclusão: o papel da aplicação não tem DELETE; a função age só no tenant do escopo.
        removed = await self._c.fetchval("SELECT erase_subject($1, $2)", subject_id, request_id)
        return int(removed)

    async def is_erased(self, tenant_id: UUID, subject_id: UUID) -> bool:
        return bool(
            await self._c.fetchval(
                "SELECT EXISTS (SELECT 1 FROM erased_subjects WHERE tenant_id = $1 AND subject_id = $2)",
                tenant_id,
                subject_id,
            )
        )


class PostgresRepository:
    """Cada chamada avulsa é uma transação curta com o escopo do tenant; o consumidor usa `unit_of_work`."""

    def __init__(self, dsn: str) -> None:
        self._dsn = dsn
        self._pool_: asyncpg.Pool | None = None

    async def start(self) -> None:
        self._pool_ = await asyncpg.create_pool(self._dsn, min_size=1, max_size=10)

    async def close(self) -> None:
        if self._pool_ is not None:
            await self._pool_.close()

    @property
    def _pool(self) -> asyncpg.Pool:
        if self._pool_ is None:
            raise RuntimeError("PostgresRepository.start() não foi chamado")
        return self._pool_

    @asynccontextmanager
    async def unit_of_work(self, tenant_id: UUID) -> AsyncIterator[_TxRepository]:
        # set_config com is_local=true: o escopo morre com a transação e não vaza pelo pool de conexões.
        async with self._pool.acquire() as conn, conn.transaction():
            await conn.execute("SELECT set_config('app.tenant_id', $1, true)", str(tenant_id))
            yield _TxRepository(conn)

    async def ping(self) -> None:
        await self._pool.fetchval("SELECT 1")

    async def claim_event(self, tenant_id: UUID, event_id: UUID) -> bool:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.claim_event(tenant_id, event_id)

    async def save_tenant_settings(self, settings: TenantSettings) -> None:
        async with self.unit_of_work(settings.tenant_id) as tx:
            await tx.save_tenant_settings(settings)

    async def get_tenant_settings(self, tenant_id: UUID) -> TenantSettings | None:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.get_tenant_settings(tenant_id)

    async def upsert_employment(self, record: EmploymentRecord) -> None:
        async with self.unit_of_work(record.tenant_id) as tx:
            await tx.upsert_employment(record)

    async def get_employment(self, tenant_id: UUID, employee_id: UUID) -> EmploymentRecord | None:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.get_employment(tenant_id, employee_id)

    async def list_employment(self, tenant_id: UUID) -> list[EmploymentRecord]:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.list_employment(tenant_id)

    async def save_assessment(self, fact: AssessmentFact) -> None:
        async with self.unit_of_work(fact.tenant_id) as tx:
            await tx.save_assessment(fact)

    async def latest_assessment(self, tenant_id: UUID, employee_id: UUID) -> AssessmentFact | None:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.latest_assessment(tenant_id, employee_id)

    async def save_nine_box(self, fact: NineBoxFact) -> None:
        async with self.unit_of_work(fact.tenant_id) as tx:
            await tx.save_nine_box(fact)

    async def list_nine_box(self, tenant_id: UUID, cycle_id: UUID) -> list[NineBoxFact]:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.list_nine_box(tenant_id, cycle_id)

    async def save_survey(self, fact: SurveyFact) -> None:
        async with self.unit_of_work(fact.tenant_id) as tx:
            await tx.save_survey(fact)

    async def get_survey(self, tenant_id: UUID, survey_id: UUID) -> SurveyFact | None:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.get_survey(tenant_id, survey_id)

    async def erase_subject(self, tenant_id: UUID, subject_id: UUID, request_id: UUID) -> int:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.erase_subject(tenant_id, subject_id, request_id)

    async def is_erased(self, tenant_id: UUID, subject_id: UUID) -> bool:
        async with self.unit_of_work(tenant_id) as tx:
            return await tx.is_erased(tenant_id, subject_id)
