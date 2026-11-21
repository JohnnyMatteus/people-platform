from fastapi.testclient import TestClient

from analytics_ai.adapters.http.api import create_app
from conftest import TENANT_A, FakeLLM, make_deps, people


async def test_turnover_endpoint_aggregates_and_suppresses(repo, tokens):
    for r in people(TENANT_A, "ops", 10, terminated=2) + people(TENANT_A, "ti", 2):
        await repo.upsert_employment(r)
    c = TestClient(create_app(make_deps(repo, tokens, FakeLLM())))
    body = c.get(
        "/v1/analytics/turnover?group_by=area&period=2026-Q3", headers={"Authorization": "Bearer manager-a"}
    ).json()
    assert [g["group"] for g in body["groups"]] == ["ops"]
    assert body["suppressed_groups"] == 1 and body["total"] is None
    assert "employee_id" not in str(body)


async def test_turnover_is_tenant_scoped(repo, tokens):
    for r in people(TENANT_A, "ops", 10):
        await repo.upsert_employment(r)
    c = TestClient(create_app(make_deps(repo, tokens, FakeLLM())))
    body = c.get(
        "/v1/analytics/turnover?group_by=area&period=2026-Q3", headers={"Authorization": "Bearer manager-b"}
    ).json()
    assert body["groups"] == [] and body["total"] is None


def test_invalid_period_and_group_by(repo, tokens):
    c = TestClient(create_app(make_deps(repo, tokens, FakeLLM())))
    h = {"Authorization": "Bearer manager-a"}
    assert c.get("/v1/analytics/turnover?group_by=area&period=abc", headers=h).status_code == 422
    assert c.get("/v1/analytics/turnover?group_by=salary&period=2026", headers=h).status_code == 422
