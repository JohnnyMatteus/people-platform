import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'
import { get, patch, post, put } from '@/lib/http'
import * as s from '@/lib/schemas/performance'

const base = '/performance/v1'

export const useMe = () => useQuery({ queryKey: ['me'], queryFn: () => get(s.me, `${base}/me`), staleTime: 5 * 60_000 })

export const useCycles = () => useQuery({ queryKey: ['cycles'], queryFn: () => get(s.cycles, `${base}/cycles`) })

export const useEmployees = (enabled: Ref<boolean> | boolean = true) =>
  useQuery({ queryKey: ['employees'], queryFn: () => get(s.employees, `${base}/employees`), enabled })

export const usePolicy = () => useQuery({ queryKey: ['policy'], queryFn: () => get(s.policy, `${base}/nine-box/policy`) })

export function useCycleSession(cycleId: Ref<string | undefined>) {
  return useQuery({
    queryKey: computed(() => ['session', cycleId.value]),
    queryFn: () => get(s.session, `${base}/cycles/${cycleId.value}/calibration-sessions/current`),
    enabled: computed(() => !!cycleId.value),
    // 404 = ciclo ainda sem sessão: é um estado da tela, não uma falha.
    retry: false,
  })
}

export function useNineBoxHistory(employeeId: Ref<string | undefined>) {
  return useQuery({
    queryKey: computed(() => ['history', employeeId.value]),
    queryFn: () => get(s.history, `${base}/employees/${employeeId.value}/nine-box-history`),
    enabled: computed(() => !!employeeId.value),
    retry: false,
  })
}

export function useProgress(cycleId: Ref<string | undefined>, enabled: Ref<boolean>) {
  return useQuery({
    queryKey: computed(() => ['progress', cycleId.value]),
    queryFn: () => get(s.progress, `${base}/cycles/${cycleId.value}/evaluations`),
    enabled: computed(() => !!cycleId.value && enabled.value),
  })
}

export function useEmployeeEvaluations(cycleId: Ref<string | undefined>, employeeId: Ref<string | undefined>) {
  return useQuery({
    queryKey: computed(() => ['evaluations', cycleId.value, employeeId.value]),
    queryFn: () => get(s.evaluationList, `${base}/cycles/${cycleId.value}/employees/${employeeId.value}/evaluations`),
    enabled: computed(() => !!cycleId.value && !!employeeId.value),
  })
}

export const useSuccessionPlans = () =>
  useQuery({ queryKey: ['succession'], queryFn: () => get(s.successionPlans, `${base}/critical-positions`) })

export function usePdi(employeeId: Ref<string | undefined>) {
  return useQuery({
    queryKey: computed(() => ['pdi', employeeId.value]),
    queryFn: () => get(s.pdi, `${base}/employees/${employeeId.value}/pdi`),
    enabled: computed(() => !!employeeId.value),
  })
}

export function useReadinessSuggestion(positionId: Ref<string | undefined>, employeeId: Ref<string | undefined>) {
  return useQuery({
    queryKey: computed(() => ['readiness', positionId.value, employeeId.value]),
    queryFn: () =>
      get(s.readinessSuggestion, `${base}/critical-positions/${positionId.value}/successors/${employeeId.value}/readiness-suggestion`),
    enabled: computed(() => !!positionId.value && !!employeeId.value),
  })
}

/** Mutação que invalida as chaves afetadas, para a tela refletir o servidor sem estado paralelo. */
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

export const useCreateCycle = mutation(
  (v: { name: string; type: s.Cycle['type'] }) => post(s.cycle, `${base}/cycles`, v),
  () => [['cycles']],
)
export const useOpenCycle = mutation((id: string) => post(s.cycle, `${base}/cycles/${id}/open`), () => [['cycles']])

export const useOpenSession = mutation(
  (cycleId: string) => post(s.session, `${base}/cycles/${cycleId}/calibration-sessions`),
  (cycleId) => [['session', cycleId], ['cycles']],
)
export const useAdjust = mutation(
  (v: { sessionId: string; cycleId: string; employee_id: string; to: s.Quadrant; justification: string }) =>
    post(s.adjustment, `${base}/calibration-sessions/${v.sessionId}/adjustments`, {
      employee_id: v.employee_id,
      to: v.to,
      justification: v.justification,
    }),
  (v) => [['session', v.cycleId]],
)
export const useCloseSession = mutation(
  (v: { sessionId: string; cycleId: string }) => post(s.session, `${base}/calibration-sessions/${v.sessionId}/close`),
  (v) => [['session', v.cycleId], ['cycles'], ['history']],
)

export const useSubmitSelf = mutation(
  (v: { cycleId: string; performance_score: string; competencies: s.CompetencyScore[]; comment?: string }) =>
    post(s.evaluation, `${base}/cycles/${v.cycleId}/evaluations/self`, {
      performance_score: v.performance_score,
      competencies: v.competencies,
      comment: v.comment,
    }),
  (v) => [['progress', v.cycleId], ['evaluations']],
)
export const useSubmitManager = mutation(
  (v: {
    cycleId: string
    employee_id: string
    performance_score: string
    potential_score: string
    competencies: s.CompetencyScore[]
    comment?: string
  }) => post(s.evaluation, `${base}/cycles/${v.cycleId}/evaluations/manager`, { ...v, cycleId: undefined }),
  (v) => [['progress', v.cycleId], ['evaluations']],
)

export const useUpsertEmployee = mutation(
  (v: s.Employee) => post(s.employee, `${base}/employees`, v),
  () => [['employees']],
)

export const useCreatePosition = mutation(
  (v: { title: string; incumbent_id?: string }) => post(s.successionPlan, `${base}/critical-positions`, v),
  () => [['succession']],
)
export const useNominate = mutation(
  (v: { positionId: string; employee_id: string; readiness: s.Readiness }) =>
    post(s.successionPlan, `${base}/critical-positions/${v.positionId}/successors`, {
      employee_id: v.employee_id,
      readiness: v.readiness,
    }),
  () => [['succession']],
)

export const useAddPdiAction = mutation(
  (v: { employeeId: string; title: string; kind: s.PdiKind; due_on: string; competency?: string }) =>
    post(s.pdiAction, `${base}/employees/${v.employeeId}/pdi/actions`, {
      title: v.title,
      kind: v.kind,
      due_on: v.due_on,
      competency: v.competency,
    }),
  (v) => [['pdi', v.employeeId]],
)
export const useUpdatePdiStatus = mutation(
  (v: { actionId: string; employeeId: string; status: s.PdiStatus }) =>
    patch(s.pdiAction, `${base}/pdi-actions/${v.actionId}`, { status: v.status }),
  (v) => [['pdi', v.employeeId]],
)

export const useSetPolicy = mutation((v: s.Policy) => put(s.policy, `${base}/nine-box/policy`, v), () => [['policy']])
