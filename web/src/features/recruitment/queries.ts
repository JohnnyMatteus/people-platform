import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'
import { get, post, put } from '@/lib/http'
import * as s from '@/lib/schemas/recruitment'

const base = '/recruitment/v1'

export const useJobs = () => useQuery({ queryKey: ['jobs'], queryFn: () => get(s.jobPage, `${base}/jobs`) })

export const useJob = (id: Ref<string>) =>
  useQuery({ queryKey: computed(() => ['job', id.value]), queryFn: () => get(s.job, `${base}/jobs/${id.value}`) })

export const useApplications = (jobId: Ref<string>) =>
  useQuery({
    queryKey: computed(() => ['applications', jobId.value]),
    queryFn: () => get(s.applicationPage, `${base}/jobs/${jobId.value}/applications`),
  })

export const useApplication = (id: Ref<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['application', id.value]),
    queryFn: () => get(s.application, `${base}/applications/${id.value}`),
    enabled: computed(() => !!id.value),
  })

export const usePipelineTemplate = () =>
  useQuery({ queryKey: ['pipeline-template'], queryFn: () => get(s.pipelineTemplate, `${base}/pipeline-template`) })

function mutation<V, R>(fn: (v: V) => Promise<R>, invalidate: (v: V) => unknown[][]) {
  return () => {
    const qc = useQueryClient()
    return useMutation({
      mutationFn: fn,
      onSuccess: async (_r, v) => {
        await Promise.all(invalidate(v).map((queryKey) => qc.invalidateQueries({ queryKey })))
      },
    })
  }
}

export const useCreateJob = mutation(
  (v: { title: string; unit?: string; stages?: string[] }) => post(s.job, `${base}/jobs`, v),
  () => [['jobs']],
)

export const useReplaceTemplate = mutation(
  (v: { stages: string[] }) => put(s.pipelineTemplate, `${base}/pipeline-template`, v),
  () => [['pipeline-template']],
)

export const useApply = mutation(
  (v: { jobId: string; name: string; email: string; resume_content_type?: string }) =>
    post(s.createdApplication, `${base}/jobs/${v.jobId}/applications`, {
      name: v.name,
      email: v.email,
      resume_content_type: v.resume_content_type,
    }),
  (v) => [['applications', v.jobId]],
)

export const useMoveStage = mutation(
  (v: { applicationId: string; jobId: string; stage: string; start_date?: string }) =>
    put(s.application, `${base}/applications/${v.applicationId}/stage`, { stage: v.stage, start_date: v.start_date }),
  (v) => [['applications', v.jobId], ['application', v.applicationId]],
)

export const useReject = mutation(
  (v: { applicationId: string; jobId: string }) => post(s.application, `${base}/applications/${v.applicationId}/reject`),
  (v) => [['applications', v.jobId], ['application', v.applicationId]],
)

export const useScheduleInterview = mutation(
  (v: { applicationId: string; kind: string; scheduled_at: string; interviewer_id?: string }) =>
    post(s.interview, `${base}/applications/${v.applicationId}/interviews`, {
      kind: v.kind,
      scheduled_at: v.scheduled_at,
      interviewer_id: v.interviewer_id,
    }),
  (v) => [['application', v.applicationId]],
)

export const useScorecard = mutation(
  (v: {
    interviewId: string
    applicationId: string
    recommendation: s.Recommendation
    ratings: { criterion: string; score: number }[]
    notes?: string
  }) =>
    post(s.scorecard, `${base}/interviews/${v.interviewId}/scorecards`, {
      recommendation: v.recommendation,
      ratings: v.ratings,
      notes: v.notes,
    }),
  (v) => [['application', v.applicationId]],
)

/** O link de download é pré-assinado e curto (5 min): buscado só quando a pessoa pede. */
export const fetchResumeUrl = (applicationId: string) => get(s.resumeLink, `${base}/applications/${applicationId}/resume`)

/** Envia o currículo direto ao S3 com a URL pré-assinada devolvida na candidatura. */
export async function uploadResume(upload: { url: string; headers: Record<string, string> }, file: File) {
  const res = await fetch(upload.url, { method: 'PUT', headers: upload.headers, body: file })
  if (!res.ok) throw new Error(`O envio do currículo falhou (${res.status}).`)
}
