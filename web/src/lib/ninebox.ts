import type { Placement, Policy, Quadrant } from './schemas/performance'

export const QUADRANTS: Record<Quadrant, { label: string; performance: 0 | 1 | 2; potential: 0 | 1 | 2 }> = {
  Q1: { label: 'Risco de desempenho', performance: 0, potential: 0 },
  Q2: { label: 'Mantenedor', performance: 1, potential: 0 },
  Q3: { label: 'Especialista confiável', performance: 2, potential: 0 },
  Q4: { label: 'Em ajuste', performance: 0, potential: 1 },
  Q5: { label: 'Núcleo sólido', performance: 1, potential: 1 },
  Q6: { label: 'Alto desempenho', performance: 2, potential: 1 },
  Q7: { label: 'Potencial travado', performance: 0, potential: 2 },
  Q8: { label: 'Potencial em ascensão', performance: 1, potential: 2 },
  Q9: { label: 'Talento-chave', performance: 2, potential: 2 },
}

/** Ordem de leitura da grade: potencial alto no topo, desempenho alto à direita. */
export const GRID_ORDER: Quadrant[] = ['Q7', 'Q8', 'Q9', 'Q4', 'Q5', 'Q6', 'Q1', 'Q2', 'Q3']

export function quadrantOf(performance: 0 | 1 | 2, potential: 0 | 1 | 2): Quadrant {
  return (Object.keys(QUADRANTS) as Quadrant[]).find(
    (q) => QUADRANTS[q].performance === performance && QUADRANTS[q].potential === potential,
  ) as Quadrant
}

export interface Axis {
  min: number
  mediumFrom: number
  highFrom: number
  max: number
}

export function axisOf(policy: Policy, axis: 'performance' | 'potential'): Axis {
  return {
    min: Number(policy.scale_min),
    mediumFrom: Number(policy[axis].medium_from),
    highFrom: Number(policy[axis].high_from),
    max: Number(policy.scale_max),
  }
}

/** Faixa (0 baixo, 1 médio, 2 alto) de uma nota; limiar inferior inclusivo, como no domínio. */
export function bandOf(score: number, a: Axis): 0 | 1 | 2 {
  if (score < a.mediumFrom) return 0
  return score < a.highFrom ? 1 : 2
}

/**
 * Posição 0..1 ao longo do eixo. As três faixas ocupam um terço cada, mesmo que os limiares do
 * tenant não dividam a escala em terços: a célula é a unidade, a nota posiciona dentro dela.
 */
export function axisPosition(score: number, a: Axis): number {
  const band = bandOf(score, a)
  const [from, to] = [[a.min, a.mediumFrom], [a.mediumFrom, a.highFrom], [a.highFrom, a.max]][band] as [number, number]
  const within = to === from ? 0.5 : Math.min(1, Math.max(0, (score - from) / (to - from)))
  return (band + within) / 3
}

export interface PlottedPoint {
  employeeId: string
  /** Percentuais da área da grade (x: desempenho, y: potencial, de baixo para cima). */
  x: number
  y: number
  origin: { x: number; y: number } | null
}

const PAD = 0.07

const clampInCell = (v: number, band: number) => {
  const lo = band / 3 + PAD / 3
  const hi = (band + 1) / 3 - PAD / 3
  return Math.min(hi, Math.max(lo, v))
}

/** Onde a pessoa estaria pela nota original (sem calibração). */
function originalSpot(p: Placement, perf: Axis, pot: Axis) {
  return {
    x: clampInCell(axisPosition(Number(p.performance_score), perf), QUADRANTS[p.original].performance),
    y: clampInCell(axisPosition(Number(p.potential_score), pot), QUADRANTS[p.original].potential),
  }
}

/**
 * Posiciona cada pessoa na célula do quadrante ATUAL. Se a calibração a moveu, ela vai ao centro
 * da faixa nova nos eixos que mudaram e mantém a posição pela nota nos que não mudaram.
 * Pontos na mesma célula que ficariam sobrepostos são afastados de forma determinística.
 */
export function plot(placements: Placement[], policy: Policy, minGap = 0.045): PlottedPoint[] {
  const perf = axisOf(policy, 'performance')
  const pot = axisOf(policy, 'potential')
  const points = [...placements]
    .sort((a, b) => a.employee_id.localeCompare(b.employee_id))
    .map((p) => {
      const origin = originalSpot(p, perf, pot)
      const now = QUADRANTS[p.current]
      const was = QUADRANTS[p.original]
      const x = now.performance === was.performance ? origin.x : (now.performance + 0.5) / 3
      const y = now.potential === was.potential ? origin.y : (now.potential + 0.5) / 3
      return { employeeId: p.employee_id, x, y, origin: p.current === p.original ? null : origin, cell: p.current }
    })

  for (let pass = 0; pass < 24; pass++) {
    let moved = false
    for (let i = 0; i < points.length; i++) {
      for (let j = i + 1; j < points.length; j++) {
        const a = points[i]!
        const b = points[j]!
        if (a.cell !== b.cell) continue
        const dx = b.x - a.x
        const dy = (b.y - a.y) * 0.85
        const d = Math.hypot(dx, dy)
        if (d >= minGap) continue
        const [ux, uy] = d === 0 ? [Math.cos(i + j), Math.sin(i + j)] : [dx / d, dy / d]
        const push = (minGap - d) / 2
        const q = QUADRANTS[a.cell]
        a.x = clampInCell(a.x - ux * push, q.performance)
        a.y = clampInCell(a.y - uy * push, q.potential)
        b.x = clampInCell(b.x + ux * push, q.performance)
        b.y = clampInCell(b.y + uy * push, q.potential)
        moved = true
      }
    }
    if (!moved) break
  }
  return points.map(({ cell: _cell, ...p }) => p)
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  const first = parts[0]!
  const last = parts.length > 1 ? parts[parts.length - 1]! : ''
  return (first.charAt(0) + last.charAt(0)).toUpperCase()
}

/** Quadrantes alcançáveis por um ajuste: no máximo `max` faixas por eixo a partir da posição ORIGINAL. */
export function reachableQuadrants(original: Quadrant, maxSteps: number): Quadrant[] {
  const o = QUADRANTS[original]
  return GRID_ORDER.filter((q) => {
    const c = QUADRANTS[q]
    return Math.abs(c.performance - o.performance) <= maxSteps && Math.abs(c.potential - o.potential) <= maxSteps
  })
}
