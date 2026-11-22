"""Cálculo de indicadores agregados com supressão de grupos pequenos (LGPD)."""

from __future__ import annotations

import calendar
import re
from collections import Counter, defaultdict
from collections.abc import Iterable
from dataclasses import dataclass
from datetime import date

from .models import UNKNOWN, EmploymentRecord, GroupBy, NineBoxFact


class InvalidPeriodError(ValueError):
    pass


_PERIOD = re.compile(r"^(\d{4})(?:-(Q[1-4]|0[1-9]|1[0-2]))?$")


def parse_period(period: str) -> tuple[date, date]:
    """Aceita `2026`, `2026-Q3` e `2026-09`. Devolve início e fim inclusivos."""
    match = _PERIOD.match(period)
    if not match:
        raise InvalidPeriodError(f"período inválido: {period!r}")
    year, part = int(match.group(1)), match.group(2)
    if part is None:
        return date(year, 1, 1), date(year, 12, 31)
    if part.startswith("Q"):
        first = (int(part[1]) - 1) * 3 + 1
        last = first + 2
    else:
        first = last = int(part)
    return date(year, first, 1), date(year, last, calendar.monthrange(year, last)[1])


def tenure_months(record: EmploymentRecord, at: date) -> int:
    end = min(at, record.terminated_on) if record.terminated_on else at
    months = (end.year - record.hired_on.year) * 12 + end.month - record.hired_on.month
    if end.day < record.hired_on.day:
        months -= 1
    return max(months, 0)


def tenure_band(months: int) -> str:
    if months < 12:
        return "0-1 ano"
    if months < 36:
        return "1-3 anos"
    if months < 60:
        return "3-5 anos"
    return "5+ anos"


def age_band(birth_year: int | None, at: date) -> str:
    if birth_year is None:
        return UNKNOWN
    age = at.year - birth_year
    for limit, label in ((25, "<25"), (35, "25-34"), (45, "35-44"), (55, "45-54")):
        if age < limit:
            return label
    return "55+"


def group_key(record: EmploymentRecord, group_by: GroupBy, at: date) -> str:
    match group_by:
        case GroupBy.AREA:
            return record.area
        case GroupBy.UNIT:
            return record.unit
        case GroupBy.GENDER:
            return record.gender
        case GroupBy.AGE_BAND:
            return age_band(record.birth_year, at) if record.birth_year is not None else record.age_band
        case GroupBy.TENURE_BAND:
            return tenure_band(tenure_months(record, at))


@dataclass(frozen=True, slots=True)
class GroupIndicators:
    group: str
    headcount_start: int
    headcount_end: int
    hires: int
    terminations: int
    turnover_rate: float
    avg_tenure_months: float


@dataclass(frozen=True, slots=True)
class TurnoverReport:
    period: str
    group_by: GroupBy
    min_group_size: int
    groups: list[GroupIndicators]
    suppressed_groups: int
    total: GroupIndicators | None


def _indicators(label: str, records: list[EmploymentRecord], start: date, end: date) -> GroupIndicators:
    day_before = date.fromordinal(start.toordinal() - 1)
    head_start = sum(r.active_on(day_before) for r in records)
    head_end = sum(r.active_on(end) for r in records)
    hires = sum(start <= r.hired_on <= end for r in records)
    terms = sum(r.terminated_on is not None and start <= r.terminated_on <= end for r in records)
    average = (head_start + head_end) / 2
    active = [r for r in records if r.active_on(end)]
    avg_tenure = sum(tenure_months(r, end) for r in active) / len(active) if active else 0.0
    return GroupIndicators(
        group=label,
        headcount_start=head_start,
        headcount_end=head_end,
        hires=hires,
        terminations=terms,
        turnover_rate=round(terms / average, 4) if average else 0.0,
        avg_tenure_months=round(avg_tenure, 1),
    )


def turnover_report(
    records: Iterable[EmploymentRecord], group_by: GroupBy, period: str, min_group_size: int
) -> TurnoverReport:
    """Agrega por grupo. O tamanho do grupo é o de pessoas presentes no período.

    Grupos abaixo de k são suprimidos. Se algum foi suprimido, o total também é omitido,
    senão ele revelaria o grupo suprimido por subtração.
    """
    start, end = parse_period(period)
    present = [r for r in records if r.active_during(start, end)]
    buckets: dict[str, list[EmploymentRecord]] = defaultdict(list)
    for record in present:
        buckets[group_key(record, group_by, end)].append(record)

    shown = [
        _indicators(label, members, start, end)
        for label, members in sorted(buckets.items())
        if len(members) >= min_group_size
    ]
    suppressed = len(buckets) - len(shown)
    total = _indicators("total", present, start, end) if present and suppressed == 0 else None
    return TurnoverReport(period, group_by, min_group_size, shown, suppressed, total)


@dataclass(frozen=True, slots=True)
class QuadrantCount:
    quadrant: str
    count: int


def nine_box_distribution(
    facts: Iterable[NineBoxFact], min_group_size: int
) -> tuple[list[QuadrantCount], int]:
    """Contagem por quadrante, suprimindo quadrantes com menos de k pessoas."""
    counts = Counter(f.quadrant for f in facts)
    shown = [QuadrantCount(q, n) for q, n in sorted(counts.items()) if n >= min_group_size]
    return shown, len(counts) - len(shown)
