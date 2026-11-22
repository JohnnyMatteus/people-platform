import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'
import { get, post, put } from '@/lib/http'
import * as s from '@/lib/schemas/learning'

const base = '/learning/v1'

export const useTracks = () => useQuery({ queryKey: ['tracks'], queryFn: () => get(s.tracks, `${base}/tracks`) })
export const useMyEnrollments = () => useQuery({ queryKey: ['my-enrollments'], queryFn: () => get(s.enrollments, `${base}/me/enrollments`) })
export const useSuggestions = (competency: Ref<string>) =>
  useQuery({
    queryKey: computed(() => ['suggestions', competency.value]),
    queryFn: () => get(s.suggestions, `${base}/me/suggestions?competency=${encodeURIComponent(competency.value)}`),
  })
export const useDefaultTrack = (enabled: Ref<boolean>) =>
  useQuery({ queryKey: ['default-track'], queryFn: () => get(s.defaultTrack, `${base}/onboarding/default-track`), enabled })
export const usePendingOnboarding = (enabled: Ref<boolean>) =>
  useQuery({ queryKey: ['pending-onboarding'], queryFn: () => get(s.pendingOnboarding, `${base}/onboarding/pending`), enabled })

function mutation<V, R>(fn: (v: V) => Promise<R>, invalidate: () => unknown[][]) {
  return () => {
    const qc = useQueryClient()
    return useMutation({
      mutationFn: fn,
      onSuccess: async () => {
        await Promise.all(invalidate().map((queryKey) => qc.invalidateQueries({ queryKey })))
      },
    })
  }
}

export const useCreateTrack = mutation(
  (v: { title: string; description?: string }) => post(s.track, `${base}/tracks`, v),
  () => [['tracks']],
)
export const useAddItem = mutation(
  (v: { trackId: string; title: string; kind: s.ItemKind; minutes: number; competency?: string }) =>
    post(s.track, `${base}/tracks/${v.trackId}/items`, { title: v.title, kind: v.kind, minutes: v.minutes, competency: v.competency }),
  () => [['tracks']],
)
export const useEnroll = mutation(
  (v: { trackId: string; employee_id?: string }) =>
    post(s.enrollment, `${base}/tracks/${v.trackId}/enrollments`, v.employee_id ? { employee_id: v.employee_id } : undefined),
  () => [['my-enrollments'], ['suggestions']],
)
export const useComplete = mutation(
  (v: { enrollmentId: string; itemId: string }) => post(s.enrollment, `${base}/me/enrollments/${v.enrollmentId}/items/${v.itemId}/complete`),
  () => [['my-enrollments'], ['suggestions']],
)
export const useSetDefaultTrack = mutation(
  (trackId: string) => put(s.defaultTrack, `${base}/onboarding/default-track`, { track_id: trackId }),
  () => [['default-track']],
)
export const useBindOnboarding = mutation(
  (v: { applicationId: string; employee_id: string }) =>
    post(s.enrollment, `${base}/onboarding/${v.applicationId}/bind`, { employee_id: v.employee_id }),
  () => [['pending-onboarding']],
)
