import { z } from 'zod'

/** Espelho de contracts/openapi/engagement-service.yaml (DTOs de adapters/http/controllers.ts). */
const uuid = z.string().uuid()

export const feedback = z.object({
  id: uuid,
  from_employee_id: uuid,
  to_employee_id: uuid,
  body: z.string(),
  created_at: z.string(),
})
export type Feedback = z.infer<typeof feedback>
export const feedbacks = z.array(feedback)

export const praise = z.object({
  id: uuid,
  from_employee_id: uuid,
  to_employee_ids: z.array(uuid),
  message: z.string(),
  values: z.array(z.string()),
  created_at: z.string(),
})
export type Praise = z.infer<typeof praise>
export const praises = z.array(praise)

export const post = z.object({
  id: uuid,
  author_id: uuid,
  title: z.string(),
  body: z.string(),
  created_at: z.string(),
  read_count: z.number().int(),
  read_by_me: z.boolean(),
})
export type Post = z.infer<typeof post>
export const posts = z.array(post)
export const receipts = z.array(z.object({ employee_id: uuid, read_at: z.string() }))

export const agendaItem = z.object({ id: uuid, text: z.string(), added_by: uuid, done: z.boolean() })
export const meeting = z.object({
  id: uuid,
  leader_id: uuid,
  participant_id: uuid,
  scheduled_for: z.string(),
  agenda: z.array(agendaItem),
})
export type Meeting = z.infer<typeof meeting>
export const meetings = z.array(meeting)
export const meetingContext = z.object({
  meeting,
  recent_feedbacks: z.array(feedback),
  recent_praises: z.array(praise),
  open_agenda: z.array(agendaItem),
})

export const survey = z.object({
  id: uuid,
  title: z.string(),
  status: z.enum(['open', 'closed']),
  questions: z.array(z.object({ id: z.string(), text: z.string() })),
  segments: z.array(z.string()),
  min_group_size: z.number().int(),
  already_answered: z.boolean().optional(),
})
export type Survey = z.infer<typeof survey>
export const surveys = z.array(survey)

export const surveyResults = z.object({
  survey_id: uuid,
  min_group_size: z.number().int(),
  participation: z.number().int(),
  suppressed: z.number().int(),
  segments: z.array(
    z.object({
      segment_key: z.string(),
      respondents: z.number().int(),
      scores: z.record(z.string(), z.number()),
      comments: z.array(z.string()),
    }),
  ),
})
export type SurveyResults = z.infer<typeof surveyResults>

export const points = z.object({
  balance: z.number(),
  ledger: z.array(z.object({ points: z.number(), reason: z.string(), created_at: z.string() })),
  badges: z.array(z.object({ code: z.string(), awarded_at: z.string() })),
})
export const leaderboard = z.array(z.object({ employee_id: uuid, points: z.number() }))
