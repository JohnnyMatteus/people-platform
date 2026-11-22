import { useQuery } from '@tanstack/vue-query'
import { computed, type Ref } from 'vue'
import { get } from '@/lib/http'
import * as s from '@/lib/schemas/analytics'

const base = '/analytics/v1'

export const useTurnover = (groupBy: Ref<s.GroupBy>, period: Ref<string>) =>
  useQuery({
    queryKey: computed(() => ['turnover', groupBy.value, period.value]),
    queryFn: () => get(s.turnoverReport, `${base}/analytics/turnover?group_by=${groupBy.value}&period=${encodeURIComponent(period.value)}`),
    enabled: computed(() => /^\d{4}(-(Q[1-4]|0[1-9]|1[0-2]))?$/.test(period.value)),
    retry: false,
  })

export const useNineBoxDistribution = (cycleId: Ref<string | undefined>) =>
  useQuery({
    queryKey: computed(() => ['nine-box-dist', cycleId.value]),
    queryFn: () => get(s.nineBoxDistribution, `${base}/analytics/nine-box?cycle_id=${cycleId.value}`),
    enabled: computed(() => !!cycleId.value),
    retry: false,
  })
