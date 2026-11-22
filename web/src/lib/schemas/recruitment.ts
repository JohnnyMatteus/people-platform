import { z } from 'zod'

/** Espelho de contracts/openapi/recruitment-service.yaml (Presenter do serviço). */
const uuid = z.string().uuid()

export const job = z.object({
  id: uuid,
  title: z.string(),
  unit: z.string().nullable().optional(),
  status: z.string(),
  stages: z.array(z.string()),
  created_at: z.string().nullable().optional(),
})
export type Job = z.infer<typeof job>
export const jobPage = z.object({ items: z.array(job), next_cursor: z.string().nullable() })

export const recommendation = z.enum(['strong_yes', 'yes', 'no', 'strong_no'])
export type Recommendation = z.infer<typeof recommendation>

export const scorecard = z.object({
  id: uuid,
  interview_id: uuid,
  evaluator_id: uuid,
  recommendation,
  ratings: z.array(z.object({ criterion: z.string(), score: z.number().int() })),
  notes: z.string().nullable().optional(),
})

export const interview = z.object({
  id: uuid,
  application_id: uuid,
  kind: z.string(),
  interviewer_id: uuid.nullable().optional(),
  scheduled_at: z.string(),
  scorecards: z.array(scorecard).nullable().optional(),
})
export type Interview = z.infer<typeof interview>

export const application = z.object({
  id: uuid,
  job_id: uuid,
  stage: z.string(),
  status: z.enum(['active', 'hired', 'rejected']),
  start_date: z.string().nullable().optional(),
  candidate: z.object({ id: uuid, name: z.string(), email: z.string() }).nullable().optional(),
  interviews: z.array(interview).nullable().optional(),
})
export type Application = z.infer<typeof application>
export const applicationPage = z.object({ items: z.array(application), next_cursor: z.string().nullable() })

export const resumeUpload = z.object({
  url: z.string(),
  method: z.literal('PUT'),
  headers: z.record(z.string(), z.string()),
  expires_at: z.string(),
})
export const createdApplication = application.extend({ resume_upload: resumeUpload })
export const resumeLink = z.object({ url: z.string() })
export const pipelineTemplate = z.object({ stages: z.array(z.string()) })
