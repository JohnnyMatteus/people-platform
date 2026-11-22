import { z } from 'zod'

/** Espelho de Objective, KeyResult e CheckIn em contracts/openapi/performance-service.yaml. */
const uuid = z.string().uuid()
export const level = z.enum(['COMPANY', 'TEAM', 'INDIVIDUAL'])
export type Level = z.infer<typeof level>

export const keyResult = z.object({
  id: uuid,
  title: z.string(),
  start: z.string(),
  target: z.string(),
  current: z.string(),
  progress: z.string(),
})
export type KeyResult = z.infer<typeof keyResult>

export const objective = z.object({
  id: uuid,
  level,
  owner_id: uuid.nullable().optional(),
  owner_name: z.string().nullable().optional(),
  parent_id: uuid.nullable().optional(),
  title: z.string(),
  status: z.enum(['ACTIVE', 'CLOSED']),
  progress: z.string(),
  key_results: z.array(keyResult),
})
export type Objective = z.infer<typeof objective>
export const objectives = z.array(objective)

export const checkIn = z.object({
  id: uuid,
  key_result_id: uuid,
  value: z.string(),
  comment: z.string().nullable().optional(),
  created_by: uuid,
  created_at: z.string(),
  key_result: keyResult,
})
