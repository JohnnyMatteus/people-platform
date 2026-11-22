import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'
import { api, get, post } from '@/lib/http'
import * as s from '@/lib/schemas/engagement'
import { z } from 'zod'

const base = '/engagement/v1'
const none = z.unknown()

export const useFeedbacks = (withId: Ref<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['feedbacks', withId.value ?? null]),
    queryFn: () => get(s.feedbacks, withId.value ? `${base}/feedbacks?with=${withId.value}` : `${base}/feedbacks`),
  })
export const usePraises = () => useQuery({ queryKey: ['praises'], queryFn: () => get(s.praises, `${base}/praises`) })
export const usePosts = () => useQuery({ queryKey: ['posts'], queryFn: () => get(s.posts, `${base}/posts`) })
export const useMeetings = () => useQuery({ queryKey: ['meetings'], queryFn: () => get(s.meetings, `${base}/one-on-ones`) })
export const useMeetingContext = (id: Ref<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['meeting-context', id.value]),
    queryFn: () => get(s.meetingContext, `${base}/one-on-ones/${id.value}/context`),
    enabled: computed(() => !!id.value),
  })
export const useSurveys = () => useQuery({ queryKey: ['surveys'], queryFn: () => get(s.surveys, `${base}/surveys`) })
export const useSurveyResults = (id: Ref<string | undefined>, enabled: Ref<boolean>) =>
  useQuery({
    queryKey: computed(() => ['survey-results', id.value]),
    queryFn: () => get(s.surveyResults, `${base}/surveys/${id.value}/results`),
    enabled: computed(() => !!id.value && enabled.value),
    retry: false,
  })
export const usePoints = () => useQuery({ queryKey: ['points'], queryFn: () => get(s.points, `${base}/points/me`) })
export const useLeaderboard = () => useQuery({ queryKey: ['leaderboard'], queryFn: () => get(s.leaderboard, `${base}/leaderboard`) })

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

export const useGiveFeedback = mutation(
  (v: { to_employee_id: string; body: string }) => post(s.feedback, `${base}/feedbacks`, v),
  () => [['feedbacks']],
)
export const usePraise = mutation(
  (v: { to_employee_ids: string[]; message: string; values: string[] }) => post(s.praise, `${base}/praises`, v),
  () => [['praises'], ['points'], ['leaderboard']],
)
export const usePublishPost = mutation(
  (v: { title: string; body: string }) => post(s.post, `${base}/posts`, v),
  () => [['posts']],
)
// 204 sem corpo: o contrato é "não vem nada", por isso o schema aceita qualquer coisa.
export const useConfirmRead = mutation(
  (postId: string) => api(none, 'POST', `${base}/posts/${postId}/read-receipts`),
  () => [['posts']],
)
export const useSchedule = mutation(
  (v: { participant_id: string; scheduled_for: string }) => post(s.meeting, `${base}/one-on-ones`, v),
  () => [['meetings']],
)
export const useAddAgendaItem = mutation(
  (v: { meetingId: string; text: string }) => post(s.meeting, `${base}/one-on-ones/${v.meetingId}/agenda-items`, { text: v.text }),
  (v) => [['meetings'], ['meeting-context', v.meetingId]],
)
export const useCompleteAgendaItem = mutation(
  (v: { meetingId: string; itemId: string }) => post(s.meeting, `${base}/one-on-ones/${v.meetingId}/agenda-items/${v.itemId}/done`),
  (v) => [['meetings'], ['meeting-context', v.meetingId]],
)
export const useAnswerSurvey = mutation(
  (v: { surveyId: string; scores: Record<string, number>; comment?: string }) =>
    api(none, 'POST', `${base}/surveys/${v.surveyId}/responses`, { scores: v.scores, comment: v.comment }),
  () => [['surveys']],
)
export const useCreateSurvey = mutation(
  (v: { title: string; questions: { id: string; text: string }[]; segments: string[] }) => post(s.survey, `${base}/surveys`, v),
  () => [['surveys']],
)
export const useCloseSurvey = mutation(
  (surveyId: string) => post(s.survey, `${base}/surveys/${surveyId}/close`),
  (surveyId) => [['surveys'], ['survey-results', surveyId]],
)
