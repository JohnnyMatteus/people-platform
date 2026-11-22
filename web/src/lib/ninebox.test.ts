import { describe, expect, it } from 'vitest'
import { axisOf, axisPosition, bandOf, initials, plot, reachableQuadrants } from './ninebox'
import type { Placement, Policy } from './schemas/performance'

const policy: Policy = {
  scale_min: '1',
  scale_max: '5',
  performance: { medium_from: '2.5', high_from: '3.75' },
  potential: { medium_from: '2.5', high_from: '3.75' },
  max_steps_per_axis: 1,
  top_box_warning_share: '0.20',
  min_justification_length: 20,
}

const placement = (id: string, perf: string, pot: string, original: Placement['original'], current = original): Placement => ({
  employee_id: id,
  employee_name: 'Pessoa Exemplo',
  performance_score: perf,
  potential_score: pot,
  original,
  current,
})

describe('posição pela nota', () => {
  const a = axisOf(policy, 'performance')

  it('limiar é inclusivo na faixa de cima, como no domínio', () => {
    expect(bandOf(2.49, a)).toBe(0)
    expect(bandOf(2.5, a)).toBe(1)
    expect(bandOf(3.75, a)).toBe(2)
  })

  it('cada faixa ocupa um terço, mesmo com limiares fora dos terços', () => {
    expect(axisPosition(1, a)).toBeCloseTo(0)
    expect(axisPosition(2.5, a)).toBeCloseTo(1 / 3)
    expect(axisPosition(3.75, a)).toBeCloseTo(2 / 3)
    expect(axisPosition(5, a)).toBeCloseTo(1)
    const skewed = axisOf({ ...policy, performance: { medium_from: '2', high_from: '4.5' } }, 'performance')
    expect(axisPosition(3.25, skewed)).toBeCloseTo(0.5)
  })
})

describe('plot', () => {
  it('ponto calibrado para outra célula guarda a origem para desenhar o trilho', () => {
    const [p] = plot([placement('a', '4.5', '2.0', 'Q3', 'Q6')], policy)
    expect(p!.origin).not.toBeNull()
    expect(p!.y).toBeGreaterThan(1 / 3)
    expect(p!.y).toBeLessThan(2 / 3)
    expect(p!.x).toBeGreaterThan(2 / 3)
  })

  it('pessoa não calibrada não tem trilho', () => {
    expect(plot([placement('a', '3', '3', 'Q5')], policy)[0]!.origin).toBeNull()
  })

  it('pontos com a mesma nota na mesma célula não se sobrepõem', () => {
    const pts = plot(['a', 'b', 'c'].map((id) => placement(id, '3', '3', 'Q5')), policy)
    for (let i = 0; i < pts.length; i++) {
      for (let j = i + 1; j < pts.length; j++) {
        expect(Math.hypot(pts[i]!.x - pts[j]!.x, (pts[i]!.y - pts[j]!.y) * 0.85)).toBeGreaterThan(0.03)
      }
    }
  })
})

describe('ajuste', () => {
  it('limite de faixas é medido a partir do original', () => {
    expect(reachableQuadrants('Q1', 1).sort()).toEqual(['Q1', 'Q2', 'Q4', 'Q5'])
    expect(reachableQuadrants('Q5', 1)).toHaveLength(9)
  })

  it('iniciais usam primeiro e último nome', () => {
    expect(initials('Bia Maria Exemplo')).toBe('BE')
    expect(initials('Caio')).toBe('C')
  })
})
