import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { get, post } from '@/lib/http'
import * as s from '@/lib/schemas/okr'

const base = '/performance/v1'

export const useObjectives = () => useQuery({ queryKey: ['objectives'], queryFn: () => get(s.objectives, `${base}/objectives`) })

function mutation<V, R>(fn: (v: V) => Promise<R>) {
  return () => {
    const qc = useQueryClient()
    return useMutation({ mutationFn: fn, onSuccess: () => qc.invalidateQueries({ queryKey: ['objectives'] }) })
  }
}

export const useCreateObjective = mutation(
  (v: {
    level: s.Level
    title: string
    parent_id?: string
    key_results: { title: string; start: string; target: string }[]
  }) => post(s.objective, `${base}/objectives`, v),
)
export const useCheckIn = mutation((v: { keyResultId: string; value: string; comment?: string }) =>
  post(s.checkIn, `${base}/key-results/${v.keyResultId}/check-ins`, { value: v.value, comment: v.comment }),
)
export const useCloseObjective = mutation((id: string) => post(s.objective, `${base}/objectives/${id}/close`))
