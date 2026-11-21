from datetime import date

import pytest

from analytics_ai.domain.analytics import (
    InvalidPeriodError,
    age_band,
    nine_box_distribution,
    parse_period,
    tenure_band,
    turnover_report,
)
from analytics_ai.domain.models import GroupBy, NineBoxFact
from conftest import TENANT_A, people


def test_parse_period():
    assert parse_period("2026-Q3") == (date(2026, 7, 1), date(2026, 9, 30))
    assert parse_period("2026-02") == (date(2026, 2, 1), date(2026, 2, 28))
    assert parse_period("2026") == (date(2026, 1, 1), date(2026, 12, 31))
    with pytest.raises(InvalidPeriodError):
        parse_period("2026-Q5")


def test_bands():
    assert tenure_band(11) == "0-1 ano" and tenure_band(12) == "1-3 anos" and tenure_band(60) == "5+ anos"
    assert age_band(1990, date(2026, 9, 30)) == "35-44"
    assert age_band(None, date(2026, 9, 30)) == "nao-informado"


def test_turnover_by_area_computes_rates():
    records = people(TENANT_A, "ops", 10, terminated=2) + people(TENANT_A, "fin", 5)
    report = turnover_report(records, GroupBy.AREA, "2026-Q3", 3)
    ops = next(g for g in report.groups if g.group == "ops")
    assert (ops.headcount_start, ops.headcount_end, ops.terminations) == (10, 8, 2)
    assert ops.turnover_rate == round(2 / 9, 4)
    assert report.total is not None and report.total.terminations == 2


def test_small_groups_are_suppressed_and_total_omitted():
    records = people(TENANT_A, "ops", 10) + people(TENANT_A, "ti", 2, terminated=1)
    report = turnover_report(records, GroupBy.AREA, "2026-Q3", 3)
    assert [g.group for g in report.groups] == ["ops"]
    assert report.suppressed_groups == 1
    assert report.total is None  # evita recuperar "ti" por subtração


def test_hires_in_period_counted():
    report = turnover_report(people(TENANT_A, "ops", 4, hired="2026-08-10"), GroupBy.UNIT, "2026-Q3", 3)
    assert report.groups[0].hires == 4 and report.groups[0].headcount_start == 0


def test_nine_box_suppresses_small_quadrants():
    import uuid

    cycle = uuid.uuid4()
    facts = [NineBoxFact(TENANT_A, uuid.uuid4(), cycle, "Q9")] * 3 + [
        NineBoxFact(TENANT_A, uuid.uuid4(), cycle, "Q1")
    ]
    shown, suppressed = nine_box_distribution(facts, 3)
    assert [(q.quadrant, q.count) for q in shown] == [("Q9", 3)] and suppressed == 1
