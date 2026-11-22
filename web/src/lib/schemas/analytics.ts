import { z } from 'zod'

/** Espelho de contracts/openapi/analytics-ai-service.yaml. */
export const groupBy = z.enum(['area', 'unit', 'tenure_band', 'age_band', 'gender'])
export type GroupBy = z.infer<typeof groupBy>

export const indicators = z.object({
  group: z.string(),
  headcount_start: z.number().int(),
  headcount_end: z.number().int(),
  hires: z.number().int(),
  terminations: z.number().int(),
  turnover_rate: z.number(),
  avg_tenure_months: z.number(),
})
export type Indicators = z.infer<typeof indicators>

export const turnoverReport = z.object({
  period: z.string(),
  group_by: groupBy,
  min_group_size: z.number().int(),
  groups: z.array(indicators),
  suppressed_groups: z.number().int(),
  total: indicators.nullable(),
})
export type TurnoverReport = z.infer<typeof turnoverReport>

export const nineBoxDistribution = z.object({
  cycle_id: z.string().uuid(),
  min_group_size: z.number().int(),
  quadrants: z.array(z.object({ quadrant: z.string(), count: z.number().int() })),
  suppressed_quadrants: z.number().int(),
})
export type NineBoxDistribution = z.infer<typeof nineBoxDistribution>
