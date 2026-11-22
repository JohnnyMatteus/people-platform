import { z } from 'zod'

/** Espelho de contracts/openapi/learning-service.yaml. */
const uuid = z.string().uuid()
export const itemKind = z.enum(['video', 'article', 'course', 'practice'])
export type ItemKind = z.infer<typeof itemKind>

export const item = z.object({
  id: uuid,
  position: z.number().int(),
  title: z.string(),
  kind: itemKind,
  minutes: z.number().int(),
  competency: z.string().optional(),
})
export type Item = z.infer<typeof item>

export const track = z.object({ id: uuid, title: z.string(), description: z.string(), minutes: z.number().int(), items: z.array(item) })
export type Track = z.infer<typeof track>
export const tracks = z.array(track)

export const enrollment = z.object({
  id: uuid,
  track_id: uuid,
  employee_id: uuid,
  source: z.enum(['self', 'assigned', 'onboarding']),
  enrolled_at: z.string(),
  track: track.optional(),
  done_items: z.number().int().optional(),
  total_items: z.number().int().optional(),
  next_item: item.optional(),
  completed: z.record(z.string(), z.string()).optional(),
})
export type Enrollment = z.infer<typeof enrollment>
export const enrollments = z.array(enrollment)

const suggestion = z.object({ track_id: uuid, track_title: z.string(), item, reason: z.string() })
export const suggestions = z.object({ next: z.array(suggestion), for_competency: z.array(suggestion) })
export const defaultTrack = z.object({ track_id: uuid.nullable() })
export const pendingOnboarding = z.array(
  z.object({ application_id: uuid, position_title: z.string(), unit: z.string(), start_date: z.string() }),
)
