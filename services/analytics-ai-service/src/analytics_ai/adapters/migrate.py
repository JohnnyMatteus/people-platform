"""Aplica as migrações SQL em ordem, com advisory lock. Roda com o dono do esquema, nunca com o usuário da aplicação."""

from __future__ import annotations

import asyncio
import logging
import os
from importlib import resources

import asyncpg

log = logging.getLogger(__name__)
LOCK_KEY = 0x7E4A48


async def migrate(dsn: str) -> list[str]:
    """Devolve as migrações aplicadas nesta execução."""
    conn = await asyncpg.connect(dsn)
    applied: list[str] = []
    try:
        await conn.execute("SELECT pg_advisory_lock($1)", LOCK_KEY)
        await conn.execute(
            "CREATE TABLE IF NOT EXISTS schema_migrations ("
            "version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"
        )
        files = sorted(
            (
                f
                for f in resources.files("analytics_ai.adapters.migrations").iterdir()
                if f.name.endswith(".sql")
            ),
            key=lambda f: f.name,
        )
        for f in files:
            if await conn.fetchval(
                "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", f.name
            ):
                continue
            async with conn.transaction():
                await conn.execute(f.read_text(encoding="utf-8"))
                await conn.execute("INSERT INTO schema_migrations (version) VALUES ($1)", f.name)
            applied.append(f.name)
            log.info("migração aplicada: %s", f.name)
    finally:
        await conn.execute("SELECT pg_advisory_unlock($1)", LOCK_KEY)
        await conn.close()
    return applied


def run() -> None:
    logging.basicConfig(level=logging.INFO)
    dsn = os.environ.get("DATABASE_MIGRATION_URL")
    if not dsn:
        raise SystemExit("DATABASE_MIGRATION_URL é obrigatória para migrar")
    asyncio.run(migrate(dsn))


if __name__ == "__main__":
    run()
