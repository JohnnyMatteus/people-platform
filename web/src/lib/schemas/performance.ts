import { z } from 'zod'

/** Espelho de contracts/openapi/performance-service.yaml. */
export const quadrant = z.enum(['Q1', 'Q2', 'Q3', 'Q4', 'Q5', 'Q6', 'Q7', 'Q8', 'Q9'])
export type Quadrant = z.infer<typeof quadrant>

export const readiness = z.enum(['READY_NOW', 'ONE_TO_TWO_YEARS', 'THREE_PLUS_YEARS'])
export type Readiness = z.infer<typeof readiness>

const decimal = z.string()
const uuid = z.string().uuid()

export const axis = z.object({ medium_from: decimal, high_from: decimal })

export const policy = z.object({
  scale_min: decimal,
  scale_max: decimal,
  performance: axis,
  potential: axis,
  max_steps_per_axis: z.number().int(),
  top_box_warning_share: z.string(),
  min_justification_length: z.number().int(),
})
export type Policy = z.infer<typeof policy>

export const adjustment = z.object({
  employee_id: uuid,
  from: quadrant,
  to: quadrant,
  calibrated_by: uuid,
  justification: z.string(),
  at: z.string(),
})
export type Adjustment = z.infer<typeof adjustment>

export const placement = z.object({
  employee_id: uuid,
  employee_name: z.string(),
  performance_score: decimal,
  potential_score: decimal,
  original: quadrant,
  current: quadrant,
})
export type Placement = z.infer<typeof placement>

export const session = z.object({
  id: uuid,
  cycle_id: uuid,
  status: z.enum(['open', 'closed']),
  policy,
  placements: z.array(placement),
  distribution: z.record(z.string(), z.number().int()),
  warnings: z.array(z.object({ code: z.literal('TOP_BOX_ABOVE_GUIDELINE'), share: z.string(), guideline: z.string() })),
  adjustments: z.array(adjustment),
})
export type CalibrationSession = z.infer<typeof session>

export const history = z.object({
  entries: z.array(z.object({ cycle_id: uuid, quadrant, calibrated: z.boolean(), closed_at: z.string() })),
  trend: z.enum(['UP', 'DOWN', 'STABLE', 'LATERAL', 'INSUFFICIENT_DATA']),
})
export type NineBoxHistory = z.infer<typeof history>

export const cycleStatus = z.enum(['draft', 'open', 'calibration', 'closed'])
export const cycleType = z.enum(['self', 'manager', 'peers', '180', '360'])
export const cycle = z.object({ id: uuid, name: z.string(), type: cycleType, status: cycleStatus })
export type Cycle = z.infer<typeof cycle>
export const cycles = z.array(cycle)

export const me = z.object({ employee_id: uuid, tenant_id: uuid, name: z.string().optional(), roles: z.array(z.string()) })
export type Me = z.infer<typeof me>

export const employee = z.object({ id: uuid, name: z.string(), job_title: z.string(), role_since: z.string() })
export type Employee = z.infer<typeof employee>
export const employees = z.array(employee)

export const competencyScore = z.object({ competency: z.string(), score: z.number().int().min(1).max(5) })
export type CompetencyScore = z.infer<typeof competencyScore>

export const evaluation = z.object({
  id: uuid,
  cycle_id: uuid,
  employee_id: uuid,
  kind: z.enum(['self', 'manager']),
  performance_score: decimal,
  potential_score: decimal.optional(),
  competencies: z.array(competencyScore),
  comment: z.string().optional(),
  submitted_at: z.string(),
})
export type Evaluation = z.infer<typeof evaluation>

export const progress = z.array(
  z.object({
    employee_id: uuid,
    employee_name: z.string(),
    self_submitted: z.boolean(),
    manager_submitted: z.boolean(),
    consolidated: z.boolean(),
  }),
)
export type EvaluationProgress = z.infer<typeof progress>[number]

export const benchStrength = z.enum(['NONE', 'WEAK', 'ADEQUATE', 'STRONG'])
export const successionPlan = z.object({
  position_id: uuid,
  title: z.string(),
  incumbent_id: uuid.optional(),
  incumbent_name: z.string().optional(),
  bench_strength: benchStrength,
  successors: z.array(z.object({ employee_id: uuid, employee_name: z.string().optional(), readiness })),
})
export type SuccessionPlan = z.infer<typeof successionPlan>
export const successionPlans = z.array(successionPlan)
export const readinessSuggestion = z.object({ suggestion: readiness.nullable() })

export const pdiKind = z.enum(['COURSE', 'MENTORING', 'PROJECT', 'READING'])
export const pdiStatus = z.enum(['PLANNED', 'IN_PROGRESS', 'DONE'])
export type PdiKind = z.infer<typeof pdiKind>
export type PdiStatus = z.infer<typeof pdiStatus>
export const pdiAction = z.object({
  id: uuid,
  employee_id: uuid,
  title: z.string(),
  competency: z.string().optional(),
  kind: pdiKind,
  due_on: z.string(),
  status: pdiStatus,
})
export type PdiAction = z.infer<typeof pdiAction>
export const pdi = z.object({
  employee_id: uuid,
  gaps: z.array(z.object({ competency: z.string(), gap: z.number().int() })),
  actions: z.array(pdiAction),
  suggestions: z.array(z.object({ title: z.string(), competency: z.string().optional(), kind: pdiKind, reason: z.string() })),
})
export type Pdi = z.infer<typeof pdi>
